package crawler

import (
	"context"
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
