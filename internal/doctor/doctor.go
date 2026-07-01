// Package doctor diagnoses an already-added pinax manifest: re-crawls the
// site, re-runs the preflight content check, and reports any drift since
// the manifest was first written. Output is suitable for issue reports.
package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"pinax/internal/crawler"
	"pinax/internal/manifest"
	"pinax/internal/preflight"
	"pinax/internal/renderer"
)

// PageDriftRefuseRatio is the upper/lower bound of current/stored page
// count beyond which Diagnose flags the manifest as drifted.
const PageDriftRefuseRatio = 0.5

// Report is the structured result of a doctor run. Stable JSON shape — the
// bug-report issue template renders the same fields.
type Report struct {
	Name          string                   `json:"name"`
	BaseURL       string                   `json:"baseUrl"`
	ManifestAge   time.Duration            `json:"manifestAgeNs"`
	StoredPages   int                      `json:"storedPages"`
	StoredSource  string                   `json:"storedSource"`
	CurrentPages  int                      `json:"currentPages"`
	CurrentSource string                   `json:"currentSource"`
	Renderer      string                   `json:"renderer,omitempty"`
	Discovery     []crawler.DiscoveryProbe `json:"discovery,omitempty"`
	Preflight     *preflight.Report        `json:"preflight"`
	Healthy       bool                     `json:"healthy"`
	Reasons       []string                 `json:"reasons"`
	PinaxVersion  string                   `json:"pinaxVersion,omitempty"`
}

// Diagnose re-crawls the site behind m and compares the result against
// what's on disk. The returned Report is always non-nil.
func Diagnose(ctx context.Context, m *manifest.Manifest, pinaxVersion string) (*Report, error) {
	if m == nil {
		return nil, fmt.Errorf("doctor: nil manifest")
	}
	rep := &Report{
		Name:         m.Name,
		BaseURL:      m.BaseURL,
		ManifestAge:  time.Since(m.CrawledAt),
		StoredPages:  len(m.Pages),
		StoredSource: m.Source,
		Renderer:     m.Renderer,
		PinaxVersion: pinaxVersion,
		Reasons:      []string{},
	}
	res, err := crawler.Crawl(ctx, m.BaseURL, crawler.DefaultOptions())
	if err != nil {
		rep.Reasons = append(rep.Reasons, fmt.Sprintf("re-crawl failed: %v", err))
		return rep, nil
	}
	rep.CurrentPages = len(res.Pages)
	rep.CurrentSource = res.Source
	rep.Discovery = res.Discovery
	// Route the preflight sample through the renderer when the manifest
	// says so — otherwise every SPA-backed manifest would show as
	// unhealthy because plain-HTTP probes return empty shells.
	preOpts := preflight.Options{}
	if m.Renderer != "" {
		if r, rerr := buildDoctorRenderer(m.Renderer); rerr == nil {
			preOpts.Renderer = r
		} else {
			rep.Reasons = append(rep.Reasons,
				fmt.Sprintf("manifest requires renderer %q but %v", m.Renderer, rerr))
		}
	}
	rep.Preflight = preflight.Check(ctx, res.Pages, preOpts)

	if rep.StoredPages > 0 {
		ratio := float64(rep.CurrentPages) / float64(rep.StoredPages)
		if ratio < PageDriftRefuseRatio {
			rep.Reasons = append(rep.Reasons,
				fmt.Sprintf("page count dropped %.0f%% (stored %d, current %d) — re-crawl with 'pinax refresh %s'",
					100*(1-ratio), rep.StoredPages, rep.CurrentPages, m.Name))
		} else if ratio > 1/PageDriftRefuseRatio {
			rep.Reasons = append(rep.Reasons,
				fmt.Sprintf("page count grew %.0fx (stored %d, current %d) — re-crawl with 'pinax refresh %s'",
					ratio, rep.StoredPages, rep.CurrentPages, m.Name))
		}
	}
	if rep.StoredSource != "" && rep.CurrentSource != "" && rep.StoredSource != rep.CurrentSource {
		rep.Reasons = append(rep.Reasons,
			fmt.Sprintf("discovery source changed: %s → %s", rep.StoredSource, rep.CurrentSource))
	}
	if rep.Preflight != nil && rep.Preflight.ShouldRefuse {
		rep.Reasons = append(rep.Reasons, rep.Preflight.Reasons...)
	}
	rep.Healthy = len(rep.Reasons) == 0
	return rep, nil
}

// FormatText renders a human-readable report to w.
func (r *Report) FormatText(w io.Writer) {
	fmt.Fprintf(w, "pinax doctor: %s\n", r.Name)
	fmt.Fprintf(w, "  base URL:        %s\n", r.BaseURL)
	fmt.Fprintf(w, "  manifest age:    %s\n", truncDuration(r.ManifestAge))
	fmt.Fprintf(w, "  stored pages:    %d (via %s)\n", r.StoredPages, r.StoredSource)
	fmt.Fprintf(w, "  current pages:   %d (via %s)\n", r.CurrentPages, r.CurrentSource)
	if r.Renderer != "" {
		fmt.Fprintf(w, "  renderer:        %s\n", r.Renderer)
	}
	if r.Preflight != nil {
		fmt.Fprintf(w, "  mean prose:      %d chars\n", r.Preflight.MeanProseLen)
		fmt.Fprintf(w, "  mean text/html:  %.3f\n", r.Preflight.MeanRatio)
	}
	if len(r.Discovery) > 0 {
		fmt.Fprintln(w, "  discovery:")
		for _, p := range r.Discovery {
			marker := " "
			if p.Used {
				marker = "*"
			}
			line := fmt.Sprintf("    %s %-13s %s", marker, p.Strategy, p.Status)
			if p.Pages > 0 {
				line += fmt.Sprintf(" (%d pages)", p.Pages)
			}
			if p.URL != "" {
				line += "  " + p.URL
			}
			fmt.Fprintln(w, line)
		}
	}
	if r.Healthy {
		fmt.Fprintln(w, "  status:          OK")
		return
	}
	fmt.Fprintln(w, "  status:          UNHEALTHY")
	for _, reason := range r.Reasons {
		fmt.Fprintf(w, "  - %s\n", reason)
	}
}

// FormatJSON renders r as indented JSON suitable for pasting into a bug report.
func (r *Report) FormatJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func truncDuration(d time.Duration) time.Duration {
	switch {
	case d > time.Hour:
		return d.Truncate(time.Minute)
	case d > time.Minute:
		return d.Truncate(time.Second)
	default:
		return d.Truncate(time.Millisecond)
	}
}

func buildDoctorRenderer(name string) (renderer.Renderer, error) {
	switch name {
	case renderer.NameJina:
		return renderer.NewJina(renderer.DefaultOptions())
	default:
		return nil, fmt.Errorf("unknown renderer %q", name)
	}
}
