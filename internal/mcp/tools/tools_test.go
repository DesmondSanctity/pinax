package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"pinax/internal/cache"
	"pinax/internal/crawler"
	"pinax/internal/manifest"
	"pinax/internal/mcp/tools"
)

func testManifest(pages ...crawler.Page) *manifest.Manifest {
	return &manifest.Manifest{Name: "x", BaseURL: "https://e.com", Pages: pages}
}

func openCache(t *testing.T) *cache.PageCache {
	t.Helper()
	c, err := cache.Open(filepath.Join(t.TempDir(), "c.db"), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func callTool(t *testing.T, fn func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	var req mcp.CallToolRequest
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := fn(context.Background(), req)
	if err != nil {
		t.Fatalf("handler err: %v", err)
	}
	return res
}

func toolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatal("empty result")
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("first content not text: %T", res.Content[0])
	}
	return tc.Text
}

// ---- list_sections ----

func TestListSections_GroupsByCategory(t *testing.T) {
	m := testManifest(
		crawler.Page{URL: "https://e.com/intro", Title: "Intro", Section: "/"},
		crawler.Page{URL: "https://e.com/functions/query", Title: "Query", Section: "functions"},
		crawler.Page{URL: "https://e.com/functions/mutation", Title: "Mutation", Section: "functions"},
	)
	d := tools.New(m, nil)
	res := callTool(t, d.ListSections, "list_sections", nil)

	var out []tools.SectionSummary
	if err := json.Unmarshal([]byte(toolText(t, res)), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 sections, got %d: %+v", len(out), out)
	}
	for _, s := range out {
		if s.Name == "functions" && s.PageCount != 2 {
			t.Errorf("functions should have 2 pages, got %d", s.PageCount)
		}
	}
}

// ---- search_pages ----

func TestSearchPages_FuzzyMatch(t *testing.T) {
	m := testManifest(
		crawler.Page{URL: "https://e.com/functions/query", Title: "Query Functions"},
		crawler.Page{URL: "https://e.com/functions/mutation", Title: "Mutations"},
		crawler.Page{URL: "https://e.com/auth/setup", Title: "Auth Setup"},
	)
	d := tools.New(m, nil)
	res := callTool(t, d.SearchPages, "search_pages", map[string]any{"query": "query"})

	var hits []tools.SearchHit
	if err := json.Unmarshal([]byte(toolText(t, res)), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || !strings.Contains(strings.ToLower(hits[0].URL+hits[0].Title), "query") {
		t.Errorf("expected query match first, got %+v", hits)
	}
}

func TestSearchPages_EmptyQueryErrors(t *testing.T) {
	d := tools.New(testManifest(), nil)
	res := callTool(t, d.SearchPages, "search_pages", map[string]any{"query": "  "})
	if !res.IsError {
		t.Error("expected error result for blank query")
	}
}

func TestSearchPages_NaturalLanguageQuery(t *testing.T) {
	m := testManifest(
		crawler.Page{URL: "https://e.com/docs/api-reference/api-keys/create-api-key", Title: "Create API key", Section: "api-reference"},
		crawler.Page{URL: "https://e.com/docs/api-reference/api-keys/list-api-keys", Title: "List API keys", Section: "api-reference"},
		crawler.Page{URL: "https://e.com/docs/api-reference/emails/send", Title: "Send Email", Section: "api-reference"},
		crawler.Page{URL: "https://e.com/docs/dashboard/billing", Title: "Billing", Section: "dashboard"},
	)
	d := tools.New(m, nil)

	res := callTool(t, d.SearchPages, "search_pages", map[string]any{"query": "api keys create"})
	var hits []tools.SearchHit
	if err := json.Unmarshal([]byte(toolText(t, res)), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit for multi-token query")
	}
	if !strings.Contains(hits[0].URL, "create-api-key") {
		t.Errorf("expected create-api-key first, got %+v", hits)
	}
	for _, h := range hits {
		low := strings.ToLower(h.URL + " " + h.Title)
		if !strings.Contains(low, "create") || !strings.Contains(low, "api") || !strings.Contains(low, "key") {
			t.Errorf("hit missing required tokens: %+v", h)
		}
	}
}

func TestSearchPages_AllTokensRequired(t *testing.T) {
	m := testManifest(
		crawler.Page{URL: "https://e.com/auth/login", Title: "Login"},
		crawler.Page{URL: "https://e.com/auth/logout", Title: "Logout"},
		crawler.Page{URL: "https://e.com/billing/invoices", Title: "Invoices"},
	)
	d := tools.New(m, nil)
	res := callTool(t, d.SearchPages, "search_pages", map[string]any{"query": "auth invoices"})
	var hits []tools.SearchHit
	_ = json.Unmarshal([]byte(toolText(t, res)), &hits)
	if len(hits) != 0 {
		t.Errorf("expected 0 hits when no page contains both tokens, got %+v", hits)
	}
}

func TestSearchPages_TypoFallback(t *testing.T) {
	m := testManifest(
		crawler.Page{URL: "https://e.com/functions/mutation", Title: "Mutations"},
		crawler.Page{URL: "https://e.com/auth/setup", Title: "Auth Setup"},
	)
	d := tools.New(m, nil)
	// "mutatons" (typo, missing 'i') is not a substring of any haystack but
	// fuzzy.MatchFold should still locate it.
	res := callTool(t, d.SearchPages, "search_pages", map[string]any{"query": "mutatons"})
	var hits []tools.SearchHit
	if err := json.Unmarshal([]byte(toolText(t, res)), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || !strings.Contains(hits[0].URL, "mutation") {
		t.Errorf("expected fuzzy fallback to find mutation page, got %+v", hits)
	}
}

func TestSearchPages_RespectsLimit(t *testing.T) {
	pages := make([]crawler.Page, 20)
	for i := range pages {
		pages[i] = crawler.Page{URL: "https://e.com/p", Title: "page"}
	}
	d := tools.New(testManifest(pages...), nil)
	res := callTool(t, d.SearchPages, "search_pages", map[string]any{"query": "page", "limit": float64(3)})
	var hits []tools.SearchHit
	_ = json.Unmarshal([]byte(toolText(t, res)), &hits)
	if len(hits) != 3 {
		t.Errorf("expected 3 hits, got %d", len(hits))
	}
}

// ---- get_section_pages ----

func TestGetSectionPages_FiltersBySection(t *testing.T) {
	m := testManifest(
		crawler.Page{URL: "https://e.com/a/1", Section: "a"},
		crawler.Page{URL: "https://e.com/a/2", Section: "a"},
		crawler.Page{URL: "https://e.com/b/1", Section: "b"},
	)
	d := tools.New(m, nil)
	res := callTool(t, d.GetSectionPages, "get_section_pages", map[string]any{"section": "a"})
	var hits []tools.SearchHit
	_ = json.Unmarshal([]byte(toolText(t, res)), &hits)
	if len(hits) != 2 {
		t.Errorf("expected 2 pages in section a, got %d", len(hits))
	}
}

func TestGetSectionPages_UnknownSectionErrors(t *testing.T) {
	d := tools.New(testManifest(), nil)
	res := callTool(t, d.GetSectionPages, "get_section_pages", map[string]any{"section": "nope"})
	if !res.IsError {
		t.Error("expected error for unknown section")
	}
}

// ---- get_page ----

func TestGetPage_FetchesAndCaches(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><main><h1>Doc</h1><p>body</p></main></body></html>`))
	}))
	defer srv.Close()

	c := openCache(t)
	d := tools.New(testManifest(), c)
	d.HTTP = srv.Client()

	res := callTool(t, d.GetPage, "get_page", map[string]any{"url": srv.URL + "/p"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", toolText(t, res))
	}
	if !strings.Contains(toolText(t, res), "Doc") {
		t.Errorf("missing extracted content: %s", toolText(t, res))
	}

	_ = callTool(t, d.GetPage, "get_page", map[string]any{"url": srv.URL + "/p"})
	if calls != 1 {
		t.Errorf("expected 1 HTTP call (second served from session cache), got %d", calls)
	}
}

func TestGetPage_404ReturnsStructuredError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	d := tools.New(testManifest(), openCache(t))
	d.HTTP = srv.Client()
	res := callTool(t, d.GetPage, "get_page", map[string]any{"url": srv.URL + "/missing"})
	if !res.IsError {
		t.Fatal("expected error result")
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(toolText(t, res)), &payload); err != nil {
		t.Fatalf("error not structured JSON: %v / %s", err, toolText(t, res))
	}
	if !strings.Contains(payload["error"], "HTTP_404") {
		t.Errorf("expected HTTP_404 in error: %v", payload)
	}
	if payload["suggestion"] == "" {
		t.Error("expected suggestion field")
	}
}

func TestGetPage_RequiresURL(t *testing.T) {
	d := tools.New(testManifest(), nil)
	res := callTool(t, d.GetPage, "get_page", nil)
	if !res.IsError {
		t.Error("expected error when url missing")
	}
}
