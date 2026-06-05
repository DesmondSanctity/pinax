package crawler

import (
	"net/url"
	"strings"
)

// CanonicalURL normalises a URL for deduplication: lowercased host, no fragment,
// no trailing slash (except root). Used by the BFS crawler to detect redirect
// cycles and treat a URL and its redirect target as the same page.
func CanonicalURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	if u.Path == "" {
		u.Path = "/"
	} else if u.Path != "/" {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	return u.String()
}
