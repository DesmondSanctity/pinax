package crawler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pinax/internal/crawler"
)

const sampleSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://docs.example.com/</loc></url>
  <url><loc>https://docs.example.com/getting-started</loc></url>
  <url><loc>https://docs.example.com/functions/query</loc></url>
  <url><loc>https://docs.example.com/functions/mutation</loc></url>
  <url><loc>https://docs.example.com/admin/users</loc></url>
  <url><loc>https://other-domain.com/page</loc></url>
</urlset>`

func TestParseSitemap_FiltersExcluded(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sitemap.xml" {
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(strings.ReplaceAll(sampleSitemap, "https://docs.example.com", srv.URL)))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	pages, err := crawler.TryParseSitemap(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != 4 {
		t.Errorf("expected 4 pages, got %d", len(pages))
	}
	for _, p := range pages {
		if crawler.IsExcluded(p.URL) {
			t.Errorf("excluded URL in results: %s", p.URL)
		}
	}
}

func TestParseSitemap_HandlesIndex(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch r.URL.Path {
		case "/sitemap.xml":
			fmt.Fprintf(w, `<sitemapindex><sitemap><loc>%s/sitemap-0.xml</loc></sitemap><sitemap><loc>%s/sitemap-1.xml</loc></sitemap></sitemapindex>`, srv.URL, srv.URL)
		case "/sitemap-0.xml":
			fmt.Fprintf(w, `<urlset><url><loc>%s/page-a</loc></url></urlset>`, srv.URL)
		case "/sitemap-1.xml":
			fmt.Fprintf(w, `<urlset><url><loc>%s/page-b</loc></url></urlset>`, srv.URL)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	pages, _ := crawler.TryParseSitemap(context.Background(), srv.URL)
	if len(pages) != 2 {
		t.Errorf("expected 2 pages from sitemap index, got %d", len(pages))
	}
}

func TestParseSitemap_ReturnsNilWhenAbsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	pages, err := crawler.TryParseSitemap(context.Background(), srv.URL)
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if pages != nil {
		t.Errorf("expected nil pages, got: %v", pages)
	}
}

// Real-world bug from resend.com/docs: sitemap.xml at root listed the whole
// site (blog, marketing pages, etc.). When baseURL has a path prefix, only
// keep entries under that prefix and derive sections relative to it.
func TestParseSitemap_FiltersToBasePath(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sitemap.xml" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<urlset>
            <url><loc>%[1]s/blog/launch</loc></url>
            <url><loc>%[1]s/about</loc></url>
            <url><loc>%[1]s/docs</loc></url>
            <url><loc>%[1]s/docs/intro</loc></url>
            <url><loc>%[1]s/docs/api-keys/create</loc></url>
            <url><loc>%[1]s/docs/api-keys/revoke</loc></url>
        </urlset>`, srv.URL)
	}))
	defer srv.Close()

	pages, err := crawler.TryParseSitemap(context.Background(), srv.URL+"/docs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != 4 {
		t.Fatalf("expected 4 /docs pages, got %d: %+v", len(pages), pages)
	}
	for _, p := range pages {
		if !strings.HasPrefix(p.URL, srv.URL+"/docs") {
			t.Errorf("page outside /docs slipped through: %s", p.URL)
		}
	}

	sections := map[string]int{}
	for _, p := range pages {
		sections[p.Section]++
	}
	if sections["api-keys"] != 2 {
		t.Errorf("expected 2 pages in section 'api-keys', got %d (all: %v)", sections["api-keys"], sections)
	}
	if sections["blog"] != 0 || sections["about"] != 0 {
		t.Errorf("non-docs sections leaked into results: %v", sections)
	}
}
