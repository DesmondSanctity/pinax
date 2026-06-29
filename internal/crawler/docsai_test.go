package crawler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"pinax/internal/crawler"
)

const agentfieldDocsAIBody = `{
  "contractVersion": "2026-03-24-v1",
  "docs": [
    {"slug": "intro", "url": "/docs/learn/intro", "markdownUrl": "/llm/docs/learn/intro", "title": "Intro"},
    {"slug": "quickstart", "url": "/docs/learn/quickstart", "markdownUrl": "/llm/docs/learn/quickstart", "title": "Quick Start"},
    {"slug": "other", "url": "/docs/build/agents", "markdownUrl": "/llm/docs/build/agents", "title": "Agents"}
  ]
}`

func TestProbeDocsAIJSON_ResolvesRelativeURLsAndScopes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/docs-ai.json" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, agentfieldDocsAIBody)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	pages, err := crawler.ProbeDocsAIJSON(context.Background(), srv.URL+"/docs/learn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages under /docs/learn scope, got %d: %#v", len(pages), pages)
	}
	wantURL := srv.URL + "/docs/learn/intro"
	if pages[0].URL != wantURL {
		t.Errorf("pages[0].URL = %q, want %q", pages[0].URL, wantURL)
	}
	wantContent := srv.URL + "/llm/docs/learn/intro"
	if pages[0].ContentURL != wantContent {
		t.Errorf("pages[0].ContentURL = %q, want %q", pages[0].ContentURL, wantContent)
	}
	if pages[0].Title != "Intro" {
		t.Errorf("pages[0].Title = %q, want %q", pages[0].Title, "Intro")
	}
}

func TestProbeDocsAIJSON_ReturnsNilWhenAbsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	pages, err := crawler.ProbeDocsAIJSON(context.Background(), srv.URL)
	if err != nil {
		t.Errorf("expected nil error for missing docs-ai.json, got %v", err)
	}
	if pages != nil {
		t.Errorf("expected nil pages, got %v", pages)
	}
}

func TestProbeDocsAIJSON_SubpathBeforeRoot(t *testing.T) {
	var mu sync.Mutex
	var order []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		order = append(order, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, _ = crawler.ProbeDocsAIJSON(context.Background(), srv.URL+"/docs")
	mu.Lock()
	defer mu.Unlock()
	if len(order) < 2 {
		t.Fatalf("expected at least 2 probes, got %d: %v", len(order), order)
	}
	if order[0] != "/docs/docs-ai.json" {
		t.Errorf("first probe = %q, want /docs/docs-ai.json", order[0])
	}
	if order[1] != "/docs-ai.json" {
		t.Errorf("second probe = %q, want /docs-ai.json", order[1])
	}
}

func TestProbeDocsAIJSONReport_RecordsProbes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/docs-ai.json" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, agentfieldDocsAIBody)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	pages, probes, err := crawler.ProbeDocsAIJSONReport(context.Background(), srv.URL+"/docs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) == 0 {
		t.Fatal("expected pages, got none")
	}
	if len(probes) != 2 {
		t.Fatalf("expected 2 probes (subpath fail + root ok), got %d", len(probes))
	}
	if probes[0].Status != "HTTP 404" {
		t.Errorf("probes[0].Status = %q, want HTTP 404", probes[0].Status)
	}
	if !probes[1].Used || probes[1].Status != "ok" {
		t.Errorf("probes[1] = %+v, want Used=true Status=ok", probes[1])
	}
	if !strings.HasSuffix(probes[1].URL, "/docs-ai.json") {
		t.Errorf("probes[1].URL = %q, want suffix /docs-ai.json", probes[1].URL)
	}
}
