package crawler

import (
	"context"
	"net/url"
	"strings"
	"time"
)

// Crawl is the top-level entrypoint. It selects a discovery strategy in priority
// order: llms.txt probe → sitemap → BFS. JS-rendered platforms can still be
// crawled if they expose llms.txt or a sitemap; only when BFS would be required
// do we surface an UnsupportedPlatformError.
func Crawl(ctx context.Context, baseURL string, opts Options) (*CrawlResult, error) {
	start := time.Now()

	detection := DetectPlatform(baseURL)

	if pages, _ := ProbeLLMSTxt(ctx, baseURL); len(pages) > 0 {
		return buildResult(pages, baseURL, string(detection.Platform), "llmstxt", start), nil
	}

	if pages, _ := TryParseSitemap(ctx, baseURL); len(pages) > 0 {
		return buildResult(pages, baseURL, string(detection.Platform), "sitemap", start), nil
	}

	if !detection.Supported {
		return nil, &UnsupportedPlatformError{Platform: string(detection.Platform)}
	}

	pages, err := BFSCrawl(ctx, baseURL, opts)
	if err != nil {
		return nil, err
	}
	return buildResult(pages, baseURL, string(detection.Platform), "bfs", start), nil
}

func buildResult(pages []Page, baseURL, platform, source string, start time.Time) *CrawlResult {
	for i := range pages {
		if pages[i].Title == "" {
			pages[i].Title = titleFromURL(pages[i].URL)
		}
	}
	return &CrawlResult{
		Pages:     pages,
		BaseURL:   baseURL,
		Platform:  platform,
		Source:    source,
		CrawledAt: time.Now().UTC(),
		Stats: CrawlStats{
			Total:     len(pages),
			Succeeded: len(pages),
			Duration:  time.Since(start),
		},
	}
}

// titleFromURL derives a human-readable title from a URL when the discovery
// source didn't provide one (typical for sitemap-only sites). Strips common
// page extensions, takes the last non-empty path segment, and turns slug
// separators into spaces. Falls back to the host when the path is empty.
func titleFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	var slug string
	for i := len(segs) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(segs[i]); s != "" {
			slug = s
			break
		}
	}
	if slug == "" {
		return u.Host
	}
	if dec, err := url.PathUnescape(slug); err == nil {
		slug = dec
	}
	for _, ext := range []string{".md", ".mdx", ".html", ".htm"} {
		slug = strings.TrimSuffix(slug, ext)
	}
	slug = strings.ReplaceAll(slug, "-", " ")
	slug = strings.ReplaceAll(slug, "_", " ")
	return strings.TrimSpace(slug)
}
