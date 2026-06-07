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
	d := tools.NewSingle(m, nil)
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
	d := tools.NewSingle(m, nil)
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
	d := tools.NewSingle(testManifest(), nil)
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
	d := tools.NewSingle(m, nil)

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
	d := tools.NewSingle(m, nil)
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
	d := tools.NewSingle(m, nil)
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
	d := tools.NewSingle(testManifest(pages...), nil)
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
	d := tools.NewSingle(m, nil)
	res := callTool(t, d.GetSectionPages, "get_section_pages", map[string]any{"section": "a"})
	var hits []tools.SearchHit
	_ = json.Unmarshal([]byte(toolText(t, res)), &hits)
	if len(hits) != 2 {
		t.Errorf("expected 2 pages in section a, got %d", len(hits))
	}
}

func TestGetSectionPages_UnknownSectionErrors(t *testing.T) {
	d := tools.NewSingle(testManifest(), nil)
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
	d := tools.NewSingle(testManifest(), c)
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

	d := tools.NewSingle(testManifest(), openCache(t))
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
	d := tools.NewSingle(testManifest(), nil)
	res := callTool(t, d.GetPage, "get_page", nil)
	if !res.IsError {
		t.Error("expected error when url missing")
	}
}

// ---- unified mode (multi-manifest routing) ----

func multiManifests() map[string]*manifest.Manifest {
	return map[string]*manifest.Manifest{
		"alpha": {Name: "alpha", BaseURL: "https://alpha.dev", Pages: []crawler.Page{
			{URL: "https://alpha.dev/intro", Title: "Alpha Intro", Section: "/"},
			{URL: "https://alpha.dev/api/auth", Title: "Alpha Auth", Section: "api"},
		}},
		"beta": {Name: "beta", BaseURL: "https://beta.dev", Pages: []crawler.Page{
			{URL: "https://beta.dev/start", Title: "Beta Start", Section: "/"},
		}},
	}
}

func TestUnified_ListDocsReturnsAll(t *testing.T) {
	d := tools.New(multiManifests(), nil)
	res := callTool(t, d.ListDocs, "list_docs", nil)
	if res.IsError {
		t.Fatalf("list_docs errored: %s", toolText(t, res))
	}
	var out []tools.DocSummary
	if err := json.Unmarshal([]byte(toolText(t, res)), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("want 2 docs, got %d", len(out))
	}
	// sortedNames orders alphabetically
	if out[0].Name != "alpha" || out[1].Name != "beta" {
		t.Errorf("unexpected order: %+v", out)
	}
	if out[0].PageCount != 2 {
		t.Errorf("alpha pageCount = %d, want 2", out[0].PageCount)
	}
}

func TestUnified_MissingDocsArgErrors(t *testing.T) {
	d := tools.New(multiManifests(), nil)
	res := callTool(t, d.ListSections, "list_sections", nil)
	if !res.IsError {
		t.Fatal("expected error when docs arg missing in multi-manifest mode")
	}
	msg := toolText(t, res)
	if !strings.Contains(msg, "alpha") || !strings.Contains(msg, "beta") {
		t.Errorf("error should list available docs: %s", msg)
	}
}

func TestUnified_UnknownDocsErrors(t *testing.T) {
	d := tools.New(multiManifests(), nil)
	res := callTool(t, d.SearchPages, "search_pages", map[string]any{
		"query": "intro",
		"docs":  "gamma",
	})
	if !res.IsError {
		t.Fatal("expected error for unknown docs")
	}
	if !strings.Contains(toolText(t, res), "not found") {
		t.Errorf("expected 'not found': %s", toolText(t, res))
	}
}

func TestUnified_RoutesToCorrectManifest(t *testing.T) {
	d := tools.New(multiManifests(), nil)
	res := callTool(t, d.SearchPages, "search_pages", map[string]any{
		"query": "start",
		"docs":  "beta",
	})
	if res.IsError {
		t.Fatalf("search_pages errored: %s", toolText(t, res))
	}
	var hits []tools.SearchHit
	if err := json.Unmarshal([]byte(toolText(t, res)), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || !strings.Contains(hits[0].URL, "beta.dev") {
		t.Errorf("expected beta.dev hit, got %+v", hits)
	}
}

func TestUnified_SingleManifestNoDocsArgWorks(t *testing.T) {
	// Single-entry map should permit omitting `docs`.
	d := tools.New(map[string]*manifest.Manifest{"only": {
		Name:    "only",
		BaseURL: "https://only.dev",
		Pages:   []crawler.Page{{URL: "https://only.dev/x", Title: "X", Section: "/"}},
	}}, nil)
	res := callTool(t, d.ListSections, "list_sections", nil)
	if res.IsError {
		t.Fatalf("expected no error in single-manifest mode: %s", toolText(t, res))
	}
}

func TestUnified_ReloadHookPicksUpNewManifest(t *testing.T) {
	state := multiManifests()
	d := tools.New(state, nil)
	d.Reload = func() (map[string]*manifest.Manifest, error) { return state, nil }

	// Initially gamma is unknown.
	res := callTool(t, d.SearchPages, "search_pages", map[string]any{
		"query": "hello",
		"docs":  "gamma",
	})
	if !res.IsError {
		t.Fatal("expected error before reload")
	}

	// Hot-add gamma; next call should resolve it.
	state["gamma"] = &manifest.Manifest{Name: "gamma", BaseURL: "https://gamma.dev", Pages: []crawler.Page{
		{URL: "https://gamma.dev/hello", Title: "Hello", Section: "/"},
	}}
	res = callTool(t, d.SearchPages, "search_pages", map[string]any{
		"query": "hello",
		"docs":  "gamma",
	})
	if res.IsError {
		t.Fatalf("expected gamma to resolve after reload: %s", toolText(t, res))
	}
}
