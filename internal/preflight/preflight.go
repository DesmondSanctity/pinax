// Package preflight samples crawled pages, fetches each, and measures
// extracted prose density. It exists to convert silent inconsistency
// ("we wrote a manifest but every page is an empty SPA shell") into a
// loud, structured refusal at `pinax add` time.
package preflight

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"pinax/internal/buildinfo"
	"pinax/internal/crawler"
	"pinax/internal/extractor"
)

// Defaults for content-density gating. Numbers come from PLATFORM_RECON.md:
// every healthy SSR docs site we tested produced ≥500 chars prose per page
// with text/html ratios in the 0.10–0.45 range. The Astro Starlight outlier
// produced ~120 chars and ratio ~0.02 on the same sample size.
const (
	DefaultSampleSize   = 20
	DefaultProseFloor   = 300
	DefaultRatioFloor   = 0.05
	DefaultFailFraction = 0.5
	DefaultConcurrency  = 4
	DefaultTimeout      = 10 * time.Second
)

// Options configures Check. Zero-value fields are filled with defaults.
type Options struct {
	SampleSize   int
	ProseFloor   int
	RatioFloor   float64
	FailFraction float64
	Concurrency  int
	Timeout      time.Duration
	UserAgent    string
	HTTPClient   *http.Client
}

func (o *Options) applyDefaults() {
	if o.SampleSize <= 0 {
		o.SampleSize = DefaultSampleSize
	}
	if o.ProseFloor <= 0 {
		o.ProseFloor = DefaultProseFloor
	}
	if o.RatioFloor <= 0 {
		o.RatioFloor = DefaultRatioFloor
	}
	if o.FailFraction <= 0 {
		o.FailFraction = DefaultFailFraction
	}
	if o.Concurrency <= 0 {
		o.Concurrency = DefaultConcurrency
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	if o.UserAgent == "" {
		o.UserAgent = buildinfo.UserAgent()
	}
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{Timeout: o.Timeout}
	}
}

// PageResult is one sampled probe.
type PageResult struct {
	URL       string  `json:"url"`
	HTMLBytes int     `json:"htmlBytes"`
	ProseLen  int     `json:"proseLen"`
	Ratio     float64 `json:"ratio"`
	Err       string  `json:"err,omitempty"`
}

// Report summarises a Check run.
type Report struct {
	SampledTotal    int          `json:"sampledTotal"`
	FetchFailures   int          `json:"fetchFailures"`
	BelowProseFloor int          `json:"belowProseFloor"`
	MeanProseLen    int          `json:"meanProseLen"`
	MedianProseLen  int          `json:"medianProseLen"`
	MeanRatio       float64      `json:"meanRatio"`
	ShouldRefuse    bool         `json:"shouldRefuse"`
	Reasons         []string     `json:"reasons,omitempty"`
	Pages           []PageResult `json:"pages,omitempty"`
}

// Check fetches a stride-spaced sample of pages, extracts each, and
// computes density stats. The returned Report is always non-nil; an empty
// `pages` slice produces a report with ShouldRefuse=false and zero samples
// (the caller can decide what to do — pinax add treats it as a refusal
// because a crawl that found no pages is itself a failure).
func Check(ctx context.Context, pages []crawler.Page, opts Options) *Report {
	opts.applyDefaults()
	rep := &Report{}
	if len(pages) == 0 {
		return rep
	}

	sample := stridedSample(pages, opts.SampleSize)
	rep.SampledTotal = len(sample)

	results := make([]PageResult, len(sample))
	sem := make(chan struct{}, opts.Concurrency)
	var wg sync.WaitGroup
	for i, p := range sample {
		i, p := i, p
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = probe(ctx, p.URL, opts)
		}()
	}
	wg.Wait()
	rep.Pages = results

	var (
		totalProse int64
		totalRatio float64
		proseLens  []int
		successful int
	)
	for _, r := range results {
		if r.Err != "" {
			rep.FetchFailures++
			continue
		}
		successful++
		totalProse += int64(r.ProseLen)
		totalRatio += r.Ratio
		proseLens = append(proseLens, r.ProseLen)
		if r.ProseLen < opts.ProseFloor {
			rep.BelowProseFloor++
		}
	}

	if successful > 0 {
		rep.MeanProseLen = int(totalProse / int64(successful))
		rep.MeanRatio = totalRatio / float64(successful)
		sort.Ints(proseLens)
		rep.MedianProseLen = proseLens[len(proseLens)/2]
	}

	// Gating: refuse if either
	//   (a) too few pages cleared the prose floor, OR
	//   (b) BOTH the mean ratio and the mean prose are low (combined signal
	//       for JS-rendered shells), OR
	//   (c) we couldn't fetch most of the sample.
	// Note: a low ratio alone is NOT enough — many healthy Mintlify/Docusaurus
	// sites ship bulky HTML shells around real, substantial content and would
	// otherwise be false-positive-refused (Resend: prose=3274 chars, ratio=0.006).
	if successful > 0 {
		failFrac := float64(rep.BelowProseFloor) / float64(successful)
		if failFrac > opts.FailFraction {
			rep.ShouldRefuse = true
			rep.Reasons = append(rep.Reasons,
				fmt.Sprintf("%d of %d sampled pages have <%d chars of prose (%.0f%%, threshold %.0f%%)",
					rep.BelowProseFloor, successful, opts.ProseFloor, 100*failFrac, 100*opts.FailFraction))
		}
		if rep.MeanRatio < opts.RatioFloor && rep.MeanProseLen < 2*opts.ProseFloor {
			rep.ShouldRefuse = true
			rep.Reasons = append(rep.Reasons,
				fmt.Sprintf("mean text/html ratio %.3f below floor %.3f AND mean prose %d below %d — looks like a JS shell",
					rep.MeanRatio, opts.RatioFloor, rep.MeanProseLen, 2*opts.ProseFloor))
		}
	}
	if rep.FetchFailures*2 > rep.SampledTotal {
		rep.ShouldRefuse = true
		rep.Reasons = append(rep.Reasons,
			fmt.Sprintf("%d of %d sampled pages failed to fetch", rep.FetchFailures, rep.SampledTotal))
	}
	return rep
}

// stridedSample returns at most n pages spread evenly across the input.
// Even spacing maximises section coverage versus picking the first n.
func stridedSample(pages []crawler.Page, n int) []crawler.Page {
	if n >= len(pages) {
		return pages
	}
	step := float64(len(pages)) / float64(n)
	out := make([]crawler.Page, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, pages[int(float64(i)*step)])
	}
	return out
}

func probe(ctx context.Context, url string, opts Options) PageResult {
	r := PageResult{URL: url}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		r.Err = err.Error()
		return r
	}
	req.Header.Set("User-Agent", opts.UserAgent)
	// Mirror the runtime get_page Accept header so content-negotiating sites
	// (Docusaurus + llms.txt, etc.) hand us the same markdown they hand the
	// MCP tool — otherwise we'd measure the HTML shell and refuse healthy sites.
	req.Header.Set("Accept", "text/markdown, text/html;q=0.9, */*;q=0.8")
	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		r.Err = err.Error()
		return r
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		r.Err = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return r
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		r.Err = err.Error()
		return r
	}
	r.HTMLBytes = len(body)

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	var prose string
	switch {
	case strings.Contains(contentType, "text/markdown"), strings.HasSuffix(strings.ToLower(url), ".md"):
		prose = extractor.FromMarkdown(url, string(body))
	default:
		prose, err = extractor.FromHTML(url, string(body))
		if err != nil {
			r.Err = err.Error()
			return r
		}
	}
	r.ProseLen = len(strings.TrimSpace(prose))
	if r.HTMLBytes > 0 {
		r.Ratio = float64(r.ProseLen) / float64(r.HTMLBytes)
	}
	return r
}

// FormatDiagnostic writes a human-readable diagnostic suitable for stderr
// output on refusal. baseURL, name, and source provide context the user
// needs to file a useful bug report.
func (r *Report) FormatDiagnostic(w io.Writer, baseURL, name, source string) {
	fmt.Fprintf(w, "pinax: refusing to add %s: content-density check failed\n", name)
	fmt.Fprintf(w, "  base URL:        %s\n", baseURL)
	fmt.Fprintf(w, "  crawl source:    %s\n", source)
	fmt.Fprintf(w, "  sampled pages:   %d (failures: %d, below %d-char floor: %d)\n",
		r.SampledTotal, r.FetchFailures, DefaultProseFloor, r.BelowProseFloor)
	fmt.Fprintf(w, "  mean prose:      %d chars\n", r.MeanProseLen)
	fmt.Fprintf(w, "  median prose:    %d chars\n", r.MedianProseLen)
	fmt.Fprintf(w, "  mean text/html:  %.3f\n", r.MeanRatio)
	for _, reason := range r.Reasons {
		fmt.Fprintf(w, "  - %s\n", reason)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Per-page probe (first 10):")
	for i, p := range r.Pages {
		if i >= 10 {
			break
		}
		if p.Err != "" {
			fmt.Fprintf(w, "  ERR  %s  (%s)\n", p.URL, p.Err)
			continue
		}
		fmt.Fprintf(w, "  %5d chars  ratio %.3f  %s\n", p.ProseLen, p.Ratio, p.URL)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "If this site really does work for humans, re-run with --no-preflight")
	fmt.Fprintln(w, "and please file an issue with this output: https://github.com/DesmondSanctity/pinax/issues/new")
}
