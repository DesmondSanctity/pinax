package crawler_test

import (
	"testing"

	"pinax/internal/crawler"
)

func TestIsExcluded(t *testing.T) {
	tests := []struct {
		url      string
		excluded bool
		reason   string
	}{
		{"https://docs.example.com/admin/users", true, "admin path"},
		{"https://docs.example.com/login", true, "login path"},
		{"https://docs.example.com/signup", true, "signup path"},
		{"https://docs.example.com/static/logo.png", true, "static asset"},
		{"https://docs.example.com/assets/app.js", true, "js asset"},
		{"https://docs.example.com/_next/static/chunk.js", true, "_next"},
		{"https://docs.example.com/api?token=abc123", true, "token param"},
		{"https://docs.example.com/fonts/inter.woff2", true, "font"},

		{"https://docs.example.com/functions/query", false, "doc page"},
		{"https://docs.example.com/auth/overview", false, "auth doc (not /login)"},
		{"https://docs.example.com/api-reference/endpoints", false, "api reference"},
		{"https://docs.example.com/", false, "root"},
		{"https://docs.example.com/getting-started", false, "getting started"},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			if got := crawler.IsExcluded(tt.url); got != tt.excluded {
				t.Errorf("IsExcluded(%q) = %v, want %v", tt.url, got, tt.excluded)
			}
		})
	}
}

func TestExtractSection(t *testing.T) {
	tests := []struct {
		url, baseURL, want string
	}{
		{"https://docs.convex.dev/functions/query", "https://docs.convex.dev", "functions"},
		{"https://docs.convex.dev/auth", "https://docs.convex.dev", "auth"},
		{"https://docs.convex.dev/", "https://docs.convex.dev", "root"},
		{"https://nextjs.org/docs/app/api-reference", "https://nextjs.org/docs", "app"},
	}
	for _, tt := range tests {
		if got := crawler.ExtractSection(tt.url, tt.baseURL); got != tt.want {
			t.Errorf("ExtractSection(%q, %q) = %q, want %q", tt.url, tt.baseURL, got, tt.want)
		}
	}
}
