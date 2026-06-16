package manifest_test

import (
	"testing"

	"pinax/internal/crawler"
	"pinax/internal/manifest"
)

func TestFallbackSearch_Substring(t *testing.T) {
	pages := []crawler.Page{
		{URL: "https://x/api/auth/login", Title: "Login API"},
		{URL: "https://x/guide/intro", Title: "Intro"},
		{URL: "https://x/api/auth/logout", Title: "Logout API"},
	}
	hits := manifest.FallbackSearch(pages, "login", 10)
	if len(hits) == 0 || pages[hits[0].DocID].URL != "https://x/api/auth/login" {
		t.Fatalf("expected login page first, got %+v", hits)
	}
}

func TestFallbackSearch_FuzzyTypo(t *testing.T) {
	pages := []crawler.Page{
		{URL: "https://x/api/auth/login", Title: "Login API"},
		{URL: "https://x/api/auth/logout", Title: "Logout API"},
	}
	// All-substring path fails; fuzzy fallback should still locate login.
	hits := manifest.FallbackSearch(pages, "logn", 10)
	if len(hits) == 0 {
		t.Fatal("fuzzy fallback returned no matches")
	}
}

func TestFallbackSearch_EmptyQuery(t *testing.T) {
	pages := []crawler.Page{{URL: "https://x/a", Title: "A"}}
	if hits := manifest.FallbackSearch(pages, "  ", 10); len(hits) != 0 {
		t.Errorf("empty query should return no hits, got %d", len(hits))
	}
}

func TestFallbackSearch_LimitRespected(t *testing.T) {
	pages := []crawler.Page{
		{URL: "https://x/a", Title: "api"},
		{URL: "https://x/b", Title: "api"},
		{URL: "https://x/c", Title: "api"},
	}
	hits := manifest.FallbackSearch(pages, "api", 2)
	if len(hits) != 2 {
		t.Errorf("limit=2 should cap at 2, got %d", len(hits))
	}
}
