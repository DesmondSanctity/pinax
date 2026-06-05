package crawler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"pinax/internal/crawler"
)

func TestBFSCrawl_DoesNotDoubleProcessRedirects(t *testing.T) {
	visited := map[string]int{}
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		visited[r.URL.Path]++
		mu.Unlock()

		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<html><body><a href="/old">Old</a><a href="/new">New</a></body></html>`))
		case "/old":
			http.Redirect(w, r, "/new", http.StatusMovedPermanently)
		case "/new":
			w.Write([]byte(`<html><head><title>New Page</title></head><body>content</body></html>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	opts := crawler.DefaultOptions()
	opts.Delay = 0
	pages, err := crawler.BFSCrawl(context.Background(), srv.URL, opts)
	if err != nil {
		t.Fatalf("BFSCrawl returned error: %v", err)
	}

	mu.Lock()
	newVisits := visited["/new"]
	mu.Unlock()
	if newVisits > 1 {
		t.Errorf("/new should be visited at most once, was visited %d times", newVisits)
	}

	newCount := 0
	for _, p := range pages {
		if strings.HasSuffix(p.URL, "/new") {
			newCount++
		}
	}
	if newCount != 1 {
		t.Errorf("expected /new exactly once in results, got %d", newCount)
	}
}
