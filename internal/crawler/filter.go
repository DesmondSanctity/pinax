package crawler

import (
	"net/url"
	"path"
	"strings"
)

// excludedPathPrefixes are URL path prefixes that almost never carry doc content.
// Note: /auth is intentionally absent — it is a common doc section name
// (Convex, Clerk, Supabase). Auth-related login endpoints are covered by
// /login, /signup, /logout, /oauth.
var excludedPathPrefixes = []string{
	"/admin",
	"/login",
	"/signup",
	"/register",
	"/account",
	"/billing",
	"/logout",
	"/oauth",
	"/_next",
	"/__",
	"/static",
	"/assets",
	"/images",
	"/fonts",
	"/css",
	"/js",
}

var excludedExtensions = []string{
	".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp",
	".pdf", ".zip", ".tar", ".gz",
	".woff", ".woff2", ".ttf", ".eot", ".otf",
	".mp4", ".mp3", ".webm", ".mov",
	".css", ".js", ".map",
}

var sessionQueryParams = []string{
	"token", "session", "access_token", "key", "auth",
}

// IsExcluded reports whether a URL should be skipped during crawl.
func IsExcluded(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	p := strings.ToLower(u.Path)
	for _, prefix := range excludedPathPrefixes {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	ext := strings.ToLower(path.Ext(p))
	for _, e := range excludedExtensions {
		if ext == e {
			return true
		}
	}
	q := u.Query()
	for _, p := range sessionQueryParams {
		if q.Get(p) != "" {
			return true
		}
	}
	return false
}

// ExtractSection returns the first path segment of pageURL relative to baseURL.
// Returns "root" when the page is at baseURL itself.
func ExtractSection(pageURL, baseURL string) string {
	trimmed := strings.TrimPrefix(pageURL, baseURL)
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" {
		return "root"
	}
	parts := strings.SplitN(trimmed, "/", 2)
	if parts[0] == "" {
		return "root"
	}
	return parts[0]
}
