package crawler

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type sitemapEntry struct {
	Loc string `xml:"loc"`
}

type urlset struct {
	URLs []sitemapEntry `xml:"url"`
}

type sitemapindex struct {
	Sitemaps []sitemapEntry `xml:"sitemap"`
}

// TryParseSitemap tries the common sitemap locations under baseURL.
// Returns (nil, nil) if no sitemap is found.
func TryParseSitemap(ctx context.Context, baseURL string) ([]Page, error) {
	origin, err := siteOrigin(baseURL)
	if err != nil {
		return nil, err
	}
	candidates := []string{
		origin + "/sitemap.xml",
		origin + "/sitemap_index.xml",
		origin + "/sitemap-0.xml",
	}
	for _, c := range candidates {
		pages, err := fetchAndParseSitemap(ctx, c, baseURL)
		if err == nil && len(pages) > 0 {
			return pages, nil
		}
	}
	return nil, nil
}

func fetchAndParseSitemap(ctx context.Context, sitemapURL, baseURL string) ([]Page, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sitemapURL, nil)
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
		return nil, fmt.Errorf("sitemap: HTTP %d at %s", resp.StatusCode, sitemapURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}

	// Try as sitemap index first.
	var idx sitemapindex
	if err := xml.NewDecoder(bytes.NewReader(body)).Decode(&idx); err == nil && len(idx.Sitemaps) > 0 {
		var all []Page
		seen := make(map[string]bool)
		for _, s := range idx.Sitemaps {
			if s.Loc == "" {
				continue
			}
			pages, _ := fetchAndParseSitemap(ctx, s.Loc, baseURL)
			for _, p := range pages {
				if !seen[p.URL] {
					seen[p.URL] = true
					all = append(all, p)
				}
			}
		}
		return all, nil
	}

	var set urlset
	if err := xml.NewDecoder(bytes.NewReader(body)).Decode(&set); err != nil {
		return nil, err
	}

	prefix, err := basePathPrefix(baseURL)
	if err != nil {
		return nil, err
	}

	var pages []Page
	for _, e := range set.URLs {
		loc := strings.TrimSpace(e.Loc)
		if loc == "" {
			continue
		}
		if !strings.HasPrefix(loc, prefix) {
			continue
		}
		if IsExcluded(loc) {
			continue
		}
		pages = append(pages, Page{
			URL:     loc,
			Section: ExtractSection(loc, prefix),
		})
	}
	return pages, nil
}
