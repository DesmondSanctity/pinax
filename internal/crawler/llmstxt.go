package crawler

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ProbeLLMSTxt looks for a llms.txt index file at the subpath first, then root.
// Returns (nil, nil) when no usable llms.txt is found — callers should fall
// through to the next discovery strategy.
func ProbeLLMSTxt(ctx context.Context, baseURL string) ([]Page, error) {
	candidates, err := llmsTxtCandidates(baseURL)
	if err != nil {
		return nil, err
	}
	for _, c := range candidates {
		pages, err := fetchAndParseLLMSTxt(ctx, c, baseURL)
		if err == nil && len(pages) > 0 {
			return pages, nil
		}
	}
	return nil, nil
}

func llmsTxtCandidates(baseURL string) ([]string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	var out []string

	path := strings.TrimSuffix(parsed.Path, "/")
	if path != "" {
		sub := *parsed
		sub.Path = path + "/llms.txt"
		sub.RawQuery = ""
		sub.Fragment = ""
		out = append(out, sub.String())
	}

	root := *parsed
	root.Path = "/llms.txt"
	root.RawQuery = ""
	root.Fragment = ""
	out = append(out, root.String())
	return out, nil
}

func fetchAndParseLLMSTxt(ctx context.Context, llmsTxtURL, baseURL string) ([]Page, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, llmsTxtURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llms.txt: HTTP %d at %s", resp.StatusCode, llmsTxtURL)
	}

	originURL, err := siteOrigin(baseURL)
	if err != nil {
		return nil, err
	}
	llmsTxtBase, err := url.Parse(llmsTxtURL)
	if err != nil {
		return nil, err
	}

	var pages []Page
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip leading bullet-list markers ("- ", "* ", "+ ") so the
		// standard llmstxt.org "- [Title](URL)" form parses the same as
		// a bare "[Title](URL)" line.
		line = trimBulletPrefix(line)

		title, linkURL := parseMarkdownLink(line)
		if linkURL == "" && strings.HasPrefix(line, "http") {
			linkURL = line
		}
		if linkURL == "" {
			continue
		}
		// Resolve relative hrefs ("/guides/intro.md", "intro.md") against
		// the llms.txt URL so the origin-prefix filter below sees an
		// absolute URL. The llmstxt.org spec allows either form.
		resolved, err := llmsTxtBase.Parse(linkURL)
		if err != nil || resolved.Scheme == "" || resolved.Host == "" {
			continue
		}
		linkURL = resolved.String()
		if !strings.HasPrefix(linkURL, originURL) {
			continue
		}
		if IsExcluded(linkURL) {
			continue
		}
		if seen[linkURL] {
			continue
		}
		seen[linkURL] = true

		pages = append(pages, Page{
			URL:     linkURL,
			Title:   title,
			Section: ExtractSection(linkURL, originURL),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, errors.New("llms.txt: no usable links found")
	}
	return pages, nil
}

// parseMarkdownLink parses a single Markdown inline link, "[Title](URL)".
// Returns ("", "") when the input is not a Markdown link.
func parseMarkdownLink(s string) (title, link string) {
	if !strings.HasPrefix(s, "[") {
		return "", ""
	}
	close := strings.Index(s, "]")
	if close < 0 {
		return "", ""
	}
	rest := s[close+1:]
	if !strings.HasPrefix(rest, "(") {
		return "", ""
	}
	closeParen := strings.Index(rest, ")")
	if closeParen < 0 {
		return "", ""
	}
	return s[1:close], rest[1:closeParen]
}

// siteOrigin returns the scheme://host portion of a URL.
func siteOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid base URL: %s", raw)
	}
	return u.Scheme + "://" + u.Host, nil
}

// trimBulletPrefix strips a single leading markdown bullet marker
// ("- ", "* ", "+ ") so "- [Title](URL)" parses the same as "[Title](URL)".
// The llmstxt.org spec uses bulleted lists under section headings.
func trimBulletPrefix(s string) string {
	if len(s) < 2 {
		return s
	}
	if (s[0] == '-' || s[0] == '*' || s[0] == '+') && s[1] == ' ' {
		return strings.TrimLeft(s[2:], " ")
	}
	return s
}

// basePathPrefix returns scheme://host + cleaned base path (without trailing
// slash) so callers can both filter URLs to the sub-tree and derive sections
// relative to it. For "https://x.dev/docs" it returns "https://x.dev/docs";
// for "https://x.dev" or "https://x.dev/" it returns "https://x.dev".
func basePathPrefix(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid base URL: %s", raw)
	}
	path := strings.TrimRight(u.Path, "/")
	return u.Scheme + "://" + u.Host + path, nil
}
