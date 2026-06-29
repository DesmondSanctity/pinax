package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// docsAIDoc is one entry in a docs-ai.json manifest. Loose schema — only the
// fields Pinax cares about. Extras (aiSummary, category, keywords, etc.) are
// ignored.
type docsAIDoc struct {
	URL         string `json:"url"`
	MarkdownURL string `json:"markdownUrl"`
	Title       string `json:"title"`
}

type docsAIManifest struct {
	Docs []docsAIDoc `json:"docs"`
}

// ProbeDocsAIJSON looks for a docs-ai.json index at the subpath first then the
// site root. Used by sites like agentfield.ai (and emerging Mintlify/Fumadocs
// formats) that expose a structured per-page Markdown index.
// Returns (nil, nil) when no usable manifest is found.
func ProbeDocsAIJSON(ctx context.Context, baseURL string) ([]Page, error) {
	pages, _, err := ProbeDocsAIJSONReport(ctx, baseURL)
	return pages, err
}

// ProbeDocsAIJSONReport is like ProbeDocsAIJSON but also returns one
// DiscoveryProbe per candidate URL attempted.
func ProbeDocsAIJSONReport(ctx context.Context, baseURL string) ([]Page, []DiscoveryProbe, error) {
	candidates, err := docsAICandidates(baseURL)
	if err != nil {
		return nil, nil, err
	}
	var probes []DiscoveryProbe
	for _, c := range candidates {
		pages, status := fetchAndParseDocsAIReport(ctx, c, baseURL)
		p := DiscoveryProbe{Strategy: "docs-ai-json", URL: c, Status: status, Pages: len(pages)}
		if len(pages) > 0 {
			p.Used = true
			probes = append(probes, p)
			return pages, probes, nil
		}
		probes = append(probes, p)
	}
	return nil, probes, nil
}

func docsAICandidates(baseURL string) ([]string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	var out []string
	path := strings.TrimSuffix(parsed.Path, "/")
	if path != "" {
		sub := *parsed
		sub.Path = path + "/docs-ai.json"
		sub.RawQuery = ""
		sub.Fragment = ""
		out = append(out, sub.String())
	}
	root := *parsed
	root.Path = "/docs-ai.json"
	root.RawQuery = ""
	root.Fragment = ""
	out = append(out, root.String())
	return out, nil
}

func fetchAndParseDocsAIReport(ctx context.Context, manifestURL, baseURL string) ([]Page, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, err.Error()
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err.Error()
	}

	var m docsAIManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, "invalid-json"
	}
	if len(m.Docs) == 0 {
		return nil, "no-docs"
	}

	manifestBase, err := url.Parse(manifestURL)
	if err != nil {
		return nil, err.Error()
	}
	originURL, err := siteOrigin(baseURL)
	if err != nil {
		return nil, err.Error()
	}
	scopePrefix, err := basePathPrefix(baseURL)
	if err != nil {
		return nil, err.Error()
	}

	var pages []Page
	seen := make(map[string]bool)
	for _, d := range m.Docs {
		if d.URL == "" {
			continue
		}
		pageURL, err := manifestBase.Parse(d.URL)
		if err != nil || pageURL.Scheme == "" || pageURL.Host == "" {
			continue
		}
		full := pageURL.String()
		// Honor the user's chosen sub-tree. The manifest probably lives at
		// the site root and lists every doc; we only want pages under the
		// base path the user actually asked for.
		if !strings.HasPrefix(full, scopePrefix) {
			continue
		}
		if IsExcluded(full) {
			continue
		}
		if seen[full] {
			continue
		}
		seen[full] = true

		contentURL := ""
		if d.MarkdownURL != "" {
			if md, err := manifestBase.Parse(d.MarkdownURL); err == nil &&
				md.Scheme != "" && md.Host != "" {
				contentURL = md.String()
			}
		}

		pages = append(pages, Page{
			URL:        full,
			ContentURL: contentURL,
			Title:      strings.TrimSpace(d.Title),
			Section:    ExtractSection(full, originURL),
		})
	}
	if len(pages) == 0 {
		return nil, "no-in-scope-docs"
	}
	return pages, "ok"
}