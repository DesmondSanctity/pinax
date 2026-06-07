package doctor_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pinax/internal/crawler"
	"pinax/internal/doctor"
	"pinax/internal/manifest"
)

// docHTML is rich enough to clear the preflight prose floor.
const docHTML = `<html><head><title>Doc</title></head><body><article>
<h1>Documentation</h1>
<p>This page contains substantial documentation describing the widget's
configuration and behavior in production. It explains configuration
options, default values, supported modes including read and write, and
the trade-offs between durability and throughput. Examples follow with
detailed explanations of when to use each mode, plus pointers to the
advanced reference for edge cases that only matter at scale.</p>
<pre><code>widget.configure({ mode: "append" })</code></pre>
</article></body></html>`

// testServer serves a sitemap listing `n` page URLs, plus docHTML for each.
func testServer(t *testing.T, n int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, _ *http.Request) {
		var sb strings.Builder
		sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
		sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&sb, "<url><loc>%s/page-%d</loc></url>\n", srv.URL, i)
		}
		sb.WriteString(`</urlset>`)
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(sb.String()))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(docHTML))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestDiagnose_HealthyManifest(t *testing.T) {
	srv := testServer(t, 8)
	m := &manifest.Manifest{
		Name: "alpha", BaseURL: srv.URL, Source: "sitemap",
		CrawledAt: time.Now().Add(-1 * time.Hour),
		Pages:     synthPages(srv.URL, 8),
	}
	rep, err := doctor.Diagnose(context.Background(), m, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Healthy {
		t.Errorf("expected healthy, reasons=%v", rep.Reasons)
	}
	if rep.StoredPages != 8 || rep.CurrentPages != 8 {
		t.Errorf("page counts off: stored=%d current=%d", rep.StoredPages, rep.CurrentPages)
	}
}

func TestDiagnose_FlagsPageDrift(t *testing.T) {
	srv := testServer(t, 2) // server now exposes only 2 pages
	m := &manifest.Manifest{
		Name: "drift", BaseURL: srv.URL, Source: "sitemap",
		CrawledAt: time.Now().Add(-72 * time.Hour),
		Pages:     synthPages(srv.URL, 20), // manifest claims 20
	}
	rep, err := doctor.Diagnose(context.Background(), m, "test")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Healthy {
		t.Errorf("expected unhealthy due to drift")
	}
	hasDrift := false
	for _, r := range rep.Reasons {
		if strings.Contains(r, "page count dropped") {
			hasDrift = true
		}
	}
	if !hasDrift {
		t.Errorf("expected page-drift reason, got %v", rep.Reasons)
	}
}

func TestDiagnose_FormatJSONStableShape(t *testing.T) {
	srv := testServer(t, 4)
	m := &manifest.Manifest{
		Name: "j", BaseURL: srv.URL, Source: "sitemap",
		CrawledAt: time.Now(),
		Pages:     synthPages(srv.URL, 4),
	}
	rep, err := doctor.Diagnose(context.Background(), m, "v0.2.0-test")
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	if err := rep.FormatJSON(&sb); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(sb.String()), &decoded); err != nil {
		t.Fatalf("JSON not valid: %v", err)
	}
	for _, key := range []string{"name", "baseUrl", "manifestAgeNs", "storedPages", "currentPages", "preflight", "healthy", "pinaxVersion"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("missing key %q in JSON output: %s", key, sb.String())
		}
	}
}

func TestDiagnose_NilManifest(t *testing.T) {
	if _, err := doctor.Diagnose(context.Background(), nil, "v0"); err == nil {
		t.Error("expected error for nil manifest")
	}
}

func synthPages(base string, n int) []crawler.Page {
	out := make([]crawler.Page, n)
	for i := 0; i < n; i++ {
		out[i] = crawler.Page{URL: fmt.Sprintf("%s/page-%d", base, i)}
	}
	return out
}
