package crawler_test

import (
	"testing"

	"pinax/internal/crawler"
)

func TestCanonicalURL(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"https://docs.example.com/page/", "https://docs.example.com/page"},
		{"https://docs.example.com/page#section", "https://docs.example.com/page"},
		{"https://DOCS.Example.COM/page", "https://docs.example.com/page"},
		{"https://docs.example.com", "https://docs.example.com/"},
	}
	for _, tt := range tests {
		if got := crawler.CanonicalURL(tt.input); got != tt.want {
			t.Errorf("CanonicalURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
