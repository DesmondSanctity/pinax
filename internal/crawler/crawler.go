// Package crawler discovers documentation pages for a site via three strategies,
// tried in priority order: llms.txt probe, sitemap, then BFS link crawl.
package crawler

import (
	"fmt"
	"time"

	"pinax/internal/buildinfo"
)

// userAgent is a function so tests can override Version via buildinfo.
func userAgent() string { return buildinfo.UserAgent() }

// Page is a single documentation URL discovered during crawl.
type Page struct {
	URL     string `json:"url"`
	Title   string `json:"title,omitempty"`
	Section string `json:"section,omitempty"`
}

// CrawlResult is the output of Crawl.
type CrawlResult struct {
	Pages     []Page     `json:"pages"`
	BaseURL   string     `json:"baseUrl"`
	Platform  string     `json:"platform"`
	Source    string     `json:"source"` // "llmstxt" | "sitemap" | "bfs"
	CrawledAt time.Time  `json:"crawledAt"`
	Stats     CrawlStats `json:"stats"`
}

// CrawlStats summarises the crawl run.
type CrawlStats struct {
	Total     int           `json:"total"`
	Succeeded int           `json:"succeeded"`
	Skipped   int           `json:"skipped"`
	Failed    int           `json:"failed"`
	Duration  time.Duration `json:"durationNs"`
}

// Options configures Crawl.
type Options struct {
	MaxPages     int
	Concurrency  int
	Delay        time.Duration
	ExcludePaths []string
	Timeout      time.Duration
}

// DefaultOptions returns conservative defaults suitable for static doc sites.
func DefaultOptions() Options {
	return Options{
		MaxPages:    2000,
		Concurrency: 2,
		Delay:       500 * time.Millisecond,
		Timeout:     10 * time.Second,
	}
}

// UnsupportedPlatformError is returned when DetectPlatform identifies a site
// that requires JavaScript rendering, which Pinax does not currently support.
type UnsupportedPlatformError struct {
	Platform string
}

func (e *UnsupportedPlatformError) Error() string {
	return fmt.Sprintf("platform %s requires JavaScript rendering, which Pinax does not currently support", e.Platform)
}
