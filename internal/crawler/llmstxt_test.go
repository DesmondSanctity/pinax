package crawler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"pinax/internal/crawler"
)

func TestProbeLLMSTxt_SubpathFirst(t *testing.T) {
	var mu sync.Mutex
	var probeOrder []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		probeOrder = append(probeOrder, r.URL.Path)
		mu.Unlock()
		if r.URL.Path == "/reference/llms.txt" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "[Query](%s/reference/query)\n", r.Host)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// Inline same-origin URL so they pass the prefix filter.
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		probeOrder = append(probeOrder, r.URL.Path)
		mu.Unlock()
		if r.URL.Path == "/reference/llms.txt" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "[Query](%s/reference/query)\n", srv.URL)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	pages, err := crawler.ProbeLLMSTxt(context.Background(), srv.URL+"/reference")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	first := ""
	if len(probeOrder) > 0 {
		first = probeOrder[0]
	}
	mu.Unlock()
	if first != "/reference/llms.txt" {
		t.Errorf("expected subpath probe first, got: %v", probeOrder)
	}
	if len(pages) == 0 {
		t.Error("expected pages from llms.txt, got none")
	}
}

func TestProbeLLMSTxt_FallsBackToRoot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/llms.txt" {
			w.Header().Set("Content-Type", "text/plain")
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/llms.txt" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "[Intro](%s/intro)\n", srv.URL)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	pages, err := crawler.ProbeLLMSTxt(context.Background(), srv.URL+"/docs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) == 0 {
		t.Error("expected pages from root llms.txt fallback, got none")
	}
}

func TestProbeLLMSTxt_ReturnsNilWhenAbsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	pages, err := crawler.ProbeLLMSTxt(context.Background(), srv.URL)
	if err != nil {
		t.Errorf("expected nil error for missing llms.txt, got: %v", err)
	}
	if pages != nil {
		t.Errorf("expected nil pages, got: %v", pages)
	}
}

func TestProbeLLMSTxt_ParsesMarkdownLinks(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/llms.txt" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w,
				"[Query Functions](%s/functions/query)\n"+
					"[Auth Overview](%s/auth/overview)\n"+
					"# This line should be ignored\n",
				srv.URL, srv.URL)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	pages, err := crawler.ProbeLLMSTxt(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}
	if pages[0].Title != "Query Functions" {
		t.Errorf("expected title 'Query Functions', got %q", pages[0].Title)
	}
}

func TestProbeLLMSTxt_ParsesBulletListFormat(t *testing.T) {
	// llmstxt.org standard: bulleted lists under section headings.
	// Regression for the apex-llms.txt fallback silently producing 0 pages
	// because "- [Title](URL)" wasn't recognised as a link line.
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/llms.txt" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w,
				"# Example llms.txt\n"+
					"\n"+
					"## Guide\n"+
					"\n"+
					"- [Intro](%s/guide/intro)\n"+
					"* [Images](%s/guide/images.md)\n"+
					"  + [Nested](%s/guide/nested)\n",
				srv.URL, srv.URL, srv.URL)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	pages, err := crawler.ProbeLLMSTxt(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("expected 3 pages from bullet-list llms.txt, got %d", len(pages))
	}
	wantTitles := []string{"Intro", "Images", "Nested"}
	for i, want := range wantTitles {
		if pages[i].Title != want {
			t.Errorf("pages[%d].Title = %q, want %q", i, pages[i].Title, want)
		}
	}
}

func TestProbeLLMSTxt_ResolvesRelativeLinks(t *testing.T) {
	// Regression for developers.buffer.com / Spectaql-style llms.txt where
	// entries are root-relative paths ("- [Title](/guides/x.md)") not
	// absolute URLs. The origin-prefix filter must see resolved URLs.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/llms.txt" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w,
				"# Example API\n"+
					"\n"+
					"## Guides\n"+
					"\n"+
					"- [Introduction](/guides/introduction.md)\n"+
					"- [Hosting Media](/guides/hosting-media.md)\n"+
					"- [Sibling](nested.md)\n")
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	pages, err := crawler.ProbeLLMSTxt(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("expected 3 pages from relative-link llms.txt, got %d", len(pages))
	}
	wantURLs := []string{
		srv.URL + "/guides/introduction.md",
		srv.URL + "/guides/hosting-media.md",
		srv.URL + "/nested.md",
	}
	for i, want := range wantURLs {
		if pages[i].URL != want {
			t.Errorf("pages[%d].URL = %q, want %q", i, pages[i].URL, want)
		}
	}
}
