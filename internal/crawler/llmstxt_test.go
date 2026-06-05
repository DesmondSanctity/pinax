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
