// Package tools implements the four MCP tools exposed by pinax:
// list_sections, search_pages, get_section_pages, and get_page.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/mark3labs/mcp-go/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"

	"pinax/internal/cache"
	"pinax/internal/crawler"
	"pinax/internal/extractor"
	"pinax/internal/manifest"
)

// Deps wires the runtime resources the tool handlers need.
type Deps struct {
	Manifest *manifest.Manifest
	Cache    *cache.PageCache
	HTTP     *http.Client

	// sessionMu protects sessionCache for concurrent tool calls within a single
	// running server process.
	sessionMu    sync.RWMutex
	sessionCache map[string]string
}

// New wires up dependencies, applying defaults.
func New(m *manifest.Manifest, c *cache.PageCache) *Deps {
	return &Deps{
		Manifest:     m,
		Cache:        c,
		HTTP:         &http.Client{Timeout: 30 * time.Second},
		sessionCache: make(map[string]string),
	}
}

// ---------- list_sections ----------

// SectionSummary describes one logical doc section.
type SectionSummary struct {
	Name        string   `json:"name"`
	PageCount   int      `json:"pageCount"`
	SamplePages []string `json:"samplePages,omitempty"`
}

// ListSectionsTool returns the tool spec for list_sections.
func ListSectionsTool() mcp.Tool {
	return mcp.NewTool("list_sections",
		mcp.WithDescription(
			"List the high-level documentation sections (categories) available "+
				"for this server, with a sample of pages per section. Call this "+
				"first to get an overview of what's available.",
		),
	)
}

// ListSections handles the list_sections call.
func (d *Deps) ListSections(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	groups := map[string][]crawler.Page{}
	for _, p := range d.Manifest.Pages {
		section := p.Section
		if section == "" {
			section = "/"
		}
		groups[section] = append(groups[section], p)
	}

	out := make([]SectionSummary, 0, len(groups))
	for name, pages := range groups {
		s := SectionSummary{Name: name, PageCount: len(pages)}
		for i := 0; i < len(pages) && i < 3; i++ {
			s.SamplePages = append(s.SamplePages, pages[i].URL)
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return jsonResult(out)
}

// ---------- search_pages ----------

// SearchHit describes one page that matched a search.
type SearchHit struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Section string `json:"section"`
}

// SearchPagesTool spec.
func SearchPagesTool() mcp.Tool {
	return mcp.NewTool("search_pages",
		mcp.WithDescription(
			"Fuzzy-search documentation pages by URL or title. Returns up to "+
				"50 best matches. Use this when the user asks about a specific "+
				"feature, API, or concept.",
		),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search text")),
		mcp.WithNumber("limit", mcp.Description("Max results, default 50")),
	)
}

// SearchPages handler.
func (d *Deps) SearchPages(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return mcp.NewToolResultError("query must not be empty"), nil
	}
	limit := req.GetInt("limit", 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	tokens := strings.Fields(strings.ToLower(q))

	type scored struct {
		page  crawler.Page
		score int
	}
	var matches []scored
	for _, p := range d.Manifest.Pages {
		// Normalize separators so multi-token queries match against URL paths
		// like /api-reference/api-keys/create-api-key.
		haystack := normalizeHaystack(p.URL + " " + p.Title)
		score, ok := scoreTokens(tokens, haystack)
		if !ok {
			continue
		}
		matches = append(matches, scored{p, score})
	}
	// Typo tolerance fallback: if every token had to substring-match and we got
	// nothing, retry with a fuzzy pass.
	if len(matches) == 0 {
		for _, p := range d.Manifest.Pages {
			haystack := normalizeHaystack(p.URL + " " + p.Title)
			score, ok := fuzzyScoreTokens(tokens, haystack)
			if !ok {
				continue
			}
			matches = append(matches, scored{p, score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score < matches[j].score })

	if len(matches) > limit {
		matches = matches[:limit]
	}
	hits := make([]SearchHit, len(matches))
	for i, m := range matches {
		hits[i] = SearchHit{URL: m.page.URL, Title: m.page.Title, Section: m.page.Section}
	}
	return jsonResult(hits)
}

// scoreTokens requires every token to appear as a substring of haystack.
// Lower score is better: it rewards tokens appearing later in the path (more
// specific pages) and slightly penalises long haystacks so a leaf page beats
// an index page that merely lists the same words.
func scoreTokens(tokens []string, haystack string) (int, bool) {
	if len(tokens) == 0 {
		return 0, false
	}
	score := 0
	for _, tok := range tokens {
		idx := strings.Index(haystack, tok)
		if idx < 0 {
			return 0, false
		}
		// Reward specificity: tokens appearing later in the URL (e.g. the leaf
		// segment) signal that this page is *about* that token. We invert the
		// position so later = lower (better) score.
		score += len(haystack) - idx
	}
	score += len(haystack)
	return score, true
}

// fuzzyScoreTokens is the typo-tolerant fallback: every token must fuzzy-match
// haystack. Lower score is better.
func fuzzyScoreTokens(tokens []string, haystack string) (int, bool) {
	if len(tokens) == 0 {
		return 0, false
	}
	total := 0
	for _, tok := range tokens {
		if !fuzzy.MatchFold(tok, haystack) {
			return 0, false
		}
		total += fuzzy.RankMatchFold(tok, haystack)
	}
	return total, true
}

// normalizeHaystack lowercases and replaces non-alphanumeric runes with spaces
// so per-token matching works across path separators, dashes and underscores.
func normalizeHaystack(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
			b[i] = c + ('a' - 'A')
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			b[i] = c
		default:
			b[i] = ' '
		}
	}
	return string(b)
}

// ---------- get_section_pages ----------

// GetSectionPagesTool spec.
func GetSectionPagesTool() mcp.Tool {
	return mcp.NewTool("get_section_pages",
		mcp.WithDescription(
			"Return every page belonging to a named section. Use after "+
				"list_sections to drill into one category.",
		),
		mcp.WithString("section", mcp.Required(), mcp.Description("Section name from list_sections")),
	)
}

// GetSectionPages handler.
func (d *Deps) GetSectionPages(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	section, err := req.RequireString("section")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var out []SearchHit
	for _, p := range d.Manifest.Pages {
		if p.Section == section || (section == "/" && p.Section == "") {
			out = append(out, SearchHit{URL: p.URL, Title: p.Title, Section: p.Section})
		}
	}
	if len(out) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("no pages found in section %q — call list_sections to see available sections", section)), nil
	}
	return jsonResult(out)
}

// ---------- get_page ----------

// GetPageTool spec.
func GetPageTool() mcp.Tool {
	return mcp.NewTool("get_page",
		mcp.WithDescription(
			"Fetch and extract the readable content of one documentation page. "+
				"Stripped of nav/headers/footers/scripts and returned as Markdown. "+
				"Cached for 24h. Pass the URL exactly as returned by search_pages "+
				"or list_sections.",
		),
		mcp.WithString("url", mcp.Required(), mcp.Description("Page URL")),
	)
}

// GetPage handler.
func (d *Deps) GetPage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	url, err := req.RequireString("url")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	url = crawler.CanonicalURL(url)

	content, err := d.fetchPage(ctx, url)
	if err != nil {
		return jsonError(err.Error(), url, suggestionFor(err)), nil
	}
	return mcp.NewToolResultText(content), nil
}

func (d *Deps) fetchPage(ctx context.Context, url string) (string, error) {
	d.sessionMu.RLock()
	if v, ok := d.sessionCache[url]; ok {
		d.sessionMu.RUnlock()
		return v, nil
	}
	d.sessionMu.RUnlock()

	if d.Cache != nil {
		if v, hit, err := d.Cache.Get(url); err == nil && hit {
			d.storeSession(url, v)
			return v, nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/markdown, text/html;q=0.9, */*;q=0.8")
	req.Header.Set("User-Agent", "pinax/1.0")

	resp, err := d.HTTP.Do(req)
	if err != nil {
		return "", &fetchError{Code: "FETCH_FAILED", Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", &fetchError{Code: fmt.Sprintf("HTTP_%d", resp.StatusCode), Err: fmt.Errorf("http status %d", resp.StatusCode)}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return "", err
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	var out string
	switch {
	case strings.Contains(contentType, "text/markdown"), strings.HasSuffix(strings.ToLower(url), ".md"):
		out = extractor.FromMarkdown(url, string(body))
	default:
		out, err = extractor.FromHTML(url, string(body))
		if err != nil {
			return "", err
		}
	}

	d.storeSession(url, out)
	if d.Cache != nil {
		_ = d.Cache.Set(url, out)
	}
	return out, nil
}

func (d *Deps) storeSession(url, content string) {
	d.sessionMu.Lock()
	d.sessionCache[url] = content
	d.sessionMu.Unlock()
}

type fetchError struct {
	Code string
	Err  error
}

func (e *fetchError) Error() string { return e.Code + ": " + e.Err.Error() }

func suggestionFor(err error) string {
	var fe *fetchError
	if errAs(err, &fe) {
		switch {
		case strings.HasPrefix(fe.Code, "HTTP_404"):
			return "Page may have moved. Try search_pages with related keywords."
		case strings.HasPrefix(fe.Code, "HTTP_5"):
			return "Upstream documentation site is currently failing. Retry shortly."
		case fe.Code == "FETCH_FAILED":
			return "Network error reaching the documentation site. Verify connectivity."
		}
	}
	return ""
}

// Tiny errors.As wrapper to avoid importing errors throughout the file.
func errAs(err error, target any) bool {
	type aser interface{ As(any) bool }
	if a, ok := err.(aser); ok {
		return a.As(target)
	}
	if fe, ok := err.(*fetchError); ok {
		if t, ok := target.(**fetchError); ok {
			*t = fe
			return true
		}
	}
	return false
}

// ---------- Register ----------

// Register adds all four tools to s, wrapping each handler with the supplied
// middleware (typically logging).
func (d *Deps) Register(s *mcpsrv.MCPServer, wrap func(mcpsrv.ToolHandlerFunc) mcpsrv.ToolHandlerFunc) {
	if wrap == nil {
		wrap = func(h mcpsrv.ToolHandlerFunc) mcpsrv.ToolHandlerFunc { return h }
	}
	s.AddTool(ListSectionsTool(), wrap(d.ListSections))
	s.AddTool(SearchPagesTool(), wrap(d.SearchPages))
	s.AddTool(GetSectionPagesTool(), wrap(d.GetSectionPages))
	s.AddTool(GetPageTool(), wrap(d.GetPage))
}

// ---------- helpers ----------

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func jsonError(msg, url, suggestion string) *mcp.CallToolResult {
	payload := map[string]string{"error": msg, "url": url}
	if suggestion != "" {
		payload["suggestion"] = suggestion
	}
	b, _ := json.Marshal(payload)
	return mcp.NewToolResultError(string(b))
}
