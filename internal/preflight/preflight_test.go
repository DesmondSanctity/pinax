package preflight_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pinax/internal/crawler"
	"pinax/internal/preflight"
)

// goodHTML mimics an SSR docs page: real <article> with substantial prose.
const goodHTML = `<html><head><title>Doc</title></head><body>
<article>
<h1>Realistic Documentation Page</h1>
<p>This page describes how to configure the widget. Configure the widget
by setting widget.config.value to a sensible default. The widget supports
multiple modes including read, write, and append, each with its own
trade-offs around durability and latency. Examples follow below with
detailed explanations of when to use each mode.</p>
<pre><code>widget.configure({ mode: "append", durability: "strict" })</code></pre>
<p>For more complex setups, refer to the advanced configuration reference
which covers every option in exhaustive detail including edge cases that
only matter in production deployments.</p>
</article></body></html>`

// shellHTML mimics a JS-rendered SPA shell with no real content.
const shellHTML = `<html><head><title>App</title>
<script src="/app.js"></script></head><body><div id="root"></div>
<script>document.write("loading...")</script></body></html>`

func TestCheck_GoodSitePasses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(goodHTML))
	}))
	defer srv.Close()

	pages := makePages(srv.URL, 10)
	rep := preflight.Check(context.Background(), pages, preflight.Options{
		SampleSize: 10,
		HTTPClient: srv.Client(),
	})

	if rep.ShouldRefuse {
		t.Errorf("good site should not be refused: reasons=%v", rep.Reasons)
	}
	if rep.MeanProseLen < 300 {
		t.Errorf("expected mean prose ≥300, got %d", rep.MeanProseLen)
	}
	if rep.SampledTotal != 10 {
		t.Errorf("expected 10 sampled, got %d", rep.SampledTotal)
	}
}

func TestCheck_ShellSiteRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(shellHTML))
	}))
	defer srv.Close()

	pages := makePages(srv.URL, 10)
	rep := preflight.Check(context.Background(), pages, preflight.Options{
		SampleSize: 10,
		HTTPClient: srv.Client(),
	})

	if !rep.ShouldRefuse {
		t.Fatal("shell site should be refused")
	}
	if rep.BelowProseFloor == 0 {
		t.Error("expected BelowProseFloor > 0 for shell pages")
	}
	if len(rep.Reasons) == 0 {
		t.Error("expected at least one refusal reason")
	}
}

func TestCheck_HandlesFetchFailures(t *testing.T) {
	// Server that 404s everything.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", 404)
	}))
	defer srv.Close()

	pages := makePages(srv.URL, 6)
	rep := preflight.Check(context.Background(), pages, preflight.Options{
		SampleSize: 6,
		HTTPClient: srv.Client(),
	})

	if !rep.ShouldRefuse {
		t.Fatal("all-404s should be refused")
	}
	if rep.FetchFailures != 6 {
		t.Errorf("FetchFailures = %d, want 6", rep.FetchFailures)
	}
}

func TestCheck_EmptyPagesReturnsZero(t *testing.T) {
	rep := preflight.Check(context.Background(), nil, preflight.Options{})
	if rep.ShouldRefuse {
		t.Error("empty input should not be flagged as refused by Check itself")
	}
	if rep.SampledTotal != 0 {
		t.Errorf("SampledTotal = %d, want 0", rep.SampledTotal)
	}
}

func TestCheck_StridedSampleCoversInput(t *testing.T) {
	// Force every page to expose its own index so we can verify spacing.
	var seen []string
	mu := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path)
		_, _ = fmt.Fprint(w, goodHTML)
	})
	srv := httptest.NewServer(mu)
	defer srv.Close()

	pages := makePages(srv.URL, 100)
	rep := preflight.Check(context.Background(), pages, preflight.Options{
		SampleSize:  10,
		HTTPClient:  srv.Client(),
		Concurrency: 1, // deterministic order for the slice
	})
	if rep.SampledTotal != 10 {
		t.Errorf("expected 10 samples, got %d", rep.SampledTotal)
	}
	// First and last sampled paths should bracket the full input (stride ≈ 10).
	if !strings.HasSuffix(rep.Pages[0].URL, "/page-0") {
		t.Errorf("first sample should be page-0, got %s", rep.Pages[0].URL)
	}
	if !strings.HasSuffix(rep.Pages[9].URL, "/page-900") {
		t.Errorf("last sample should be page-900 (index 90 × stride 10), got %s", rep.Pages[9].URL)
	}
}

func TestCheck_SampleSizeExceedsPopulation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(goodHTML))
	}))
	defer srv.Close()

	pages := makePages(srv.URL, 3)
	rep := preflight.Check(context.Background(), pages, preflight.Options{
		SampleSize: 20,
		HTTPClient: srv.Client(),
	})
	if rep.SampledTotal != 3 {
		t.Errorf("SampledTotal = %d, want 3 (all pages)", rep.SampledTotal)
	}
}

func TestFormatDiagnostic_IncludesContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(shellHTML))
	}))
	defer srv.Close()

	pages := makePages(srv.URL, 4)
	rep := preflight.Check(context.Background(), pages, preflight.Options{
		SampleSize: 4,
		HTTPClient: srv.Client(),
	})
	var sb strings.Builder
	rep.FormatDiagnostic(&sb, "https://example.com", "example", "bfs")
	out := sb.String()
	for _, want := range []string{"refusing to add example", "https://example.com", "bfs", "sampled pages:"} {
		if !strings.Contains(out, want) {
			t.Errorf("diagnostic missing %q:\n%s", want, out)
		}
	}
}

func makePages(base string, n int) []crawler.Page {
	out := make([]crawler.Page, n)
	for i := 0; i < n; i++ {
		out[i] = crawler.Page{URL: fmt.Sprintf("%s/page-%d", base, i*10)}
	}
	return out
}
