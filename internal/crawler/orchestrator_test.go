package crawler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"pinax/internal/crawler"
)

// Real-world bug from resend.com/docs: the orchestrator was bailing out with
// UnsupportedPlatformError before trying llms.txt or sitemap, even though
// JS-rendered platforms commonly expose both. Only when BFS is required
// should an unsupported platform fail.
func TestCrawl_UnsupportedPlatformStillUsesSitemap(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/docs":
			// Mintlify signature → DetectPlatform marks Supported=false.
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprintf(w, `<!doctype html><html><head>
                <meta name="generator" content="mintlify">
                <script src="/mintlify.js"></script>
            </head><body>js-rendered docs</body></html>`)
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprintf(w, `<urlset>
                <url><loc>%[1]s/docs/intro</loc></url>
                <url><loc>%[1]s/docs/api-keys</loc></url>
            </urlset>`, srv.URL)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	res, err := crawler.Crawl(context.Background(), srv.URL+"/docs", crawler.DefaultOptions())
	if err != nil {
		t.Fatalf("Crawl should succeed via sitemap, got: %v", err)
	}
	if res.Source != "sitemap" {
		t.Errorf("expected source=sitemap, got %q", res.Source)
	}
	if len(res.Pages) != 2 {
		t.Errorf("expected 2 pages from sitemap, got %d", len(res.Pages))
	}
	if res.Platform != string(crawler.PlatformMintlify) {
		t.Errorf("expected platform=mintlify, got %q", res.Platform)
	}
}

// When the unsupported platform has neither llms.txt nor a sitemap, we must
// still surface the UnsupportedPlatformError rather than attempting BFS.
func TestCrawl_UnsupportedPlatformFailsWithoutSitemap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprintln(w, `<html><head><script src="/mintlify.js"></script></head><body>x</body></html>`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := crawler.Crawl(context.Background(), srv.URL, crawler.DefaultOptions())
	var upe *crawler.UnsupportedPlatformError
	if err == nil || !errorsAs(err, &upe) {
		t.Fatalf("expected UnsupportedPlatformError, got %v", err)
	}
}

func errorsAs(err error, target **crawler.UnsupportedPlatformError) bool {
	for e := err; e != nil; {
		if upe, ok := e.(*crawler.UnsupportedPlatformError); ok {
			*target = upe
			return true
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := e.(unwrapper); ok {
			e = u.Unwrap()
			continue
		}
		return false
	}
	return false
}
