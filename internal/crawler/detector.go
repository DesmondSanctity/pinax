package crawler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

// Platform identifies the documentation framework powering a site.
type Platform string

const (
	PlatformDocusaurus  Platform = "docusaurus"
	PlatformVitePress   Platform = "vitepress"
	PlatformGitBook     Platform = "gitbook"
	PlatformReadTheDocs Platform = "readthedocs"
	PlatformMintlify    Platform = "mintlify"
	PlatformReadme      Platform = "readme"
	PlatformUnknown     Platform = "unknown"
)

// DetectionResult describes the platform powering a site and whether Pinax
// can handle it without a headless browser.
type DetectionResult struct {
	Platform  Platform
	JSRender  bool
	Supported bool
}

type platformSignal struct {
	platform Platform
	signals  []string
	jsRender bool
}

var platformSignals = []platformSignal{
	{PlatformDocusaurus, []string{"__docusaurus", "docusaurus-theme"}, false},
	{PlatformVitePress, []string{"vitepress", "__vitepress"}, false},
	{PlatformGitBook, []string{"gitbook.io", "x-gitbook", "gitbook-plugin"}, false},
	{PlatformReadTheDocs, []string{"readthedocs", "rtfd.io", "sphinx"}, false},
	{PlatformMintlify, []string{"mintlify", "mint.json"}, true},
	{PlatformReadme, []string{"hub.readme.io", "readme.io", "rdmd"}, true},
}

// DetectPlatform fetches the base URL and inspects body and headers for known
// framework markers. Unknown sites are treated as supported — best effort.
func DetectPlatform(baseURL string) DetectionResult {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return DetectionResult{Platform: PlatformUnknown, Supported: true}
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return DetectionResult{Platform: PlatformUnknown, Supported: true}
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	body := strings.ToLower(string(bodyBytes))

	var headers strings.Builder
	for k, v := range resp.Header {
		headers.WriteString(strings.ToLower(k))
		headers.WriteByte(':')
		headers.WriteString(strings.ToLower(strings.Join(v, ",")))
		headers.WriteByte('\n')
	}

	haystack := body + headers.String()
	for _, p := range platformSignals {
		for _, sig := range p.signals {
			if strings.Contains(haystack, sig) {
				return DetectionResult{
					Platform:  p.platform,
					JSRender:  p.jsRender,
					Supported: !p.jsRender,
				}
			}
		}
	}
	return DetectionResult{Platform: PlatformUnknown, Supported: true}
}
