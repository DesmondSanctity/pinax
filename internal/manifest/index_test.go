package manifest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pinax/internal/crawler"
	"pinax/internal/manifest"
)

func samplePages() []crawler.Page {
	return []crawler.Page{
		{URL: "https://docs.example.com/intro", Title: "Introduction", Section: "/"},
		{URL: "https://docs.example.com/auth/login", Title: "Login", Section: "auth"},
		{URL: "https://docs.example.com/auth/logout", Title: "Logout", Section: "auth"},
		{URL: "https://docs.example.com/auth/oauth/google", Title: "Google OAuth", Section: "auth"},
		{URL: "https://docs.example.com/api/users/create", Title: "Create User", Section: "api"},
		{URL: "https://docs.example.com/api/users/delete", Title: "Delete User", Section: "api"},
		{URL: "https://docs.example.com/api/search", Title: "Search API", Section: "api"},
		{URL: "https://docs.example.com/guides/vector-search", Title: "Vector Search Guide", Section: "guides"},
	}
}

func TestIndex_BuildPopulatesPostings(t *testing.T) {
	idx := manifest.BuildIndex(samplePages())
	if idx.DocCount != 8 {
		t.Errorf("DocCount = %d, want 8", idx.DocCount)
	}
	if idx.AvgLen <= 0 {
		t.Error("AvgLen should be > 0")
	}
	if len(idx.Postings["login"]) == 0 {
		t.Error("expected 'login' to appear in postings")
	}
	// Stopwords filtered out.
	if _, ok := idx.Postings["the"]; ok {
		t.Error("stopword 'the' should not be indexed")
	}
}

func TestIndex_EmptyManifest(t *testing.T) {
	idx := manifest.BuildIndex(nil)
	if idx.DocCount != 0 {
		t.Errorf("DocCount = %d, want 0", idx.DocCount)
	}
	if hits := idx.Search("anything", 10); hits != nil {
		t.Errorf("empty index should return nil, got %v", hits)
	}
}

func TestIndex_TokenizeStripsAndLowercases(t *testing.T) {
	idx := manifest.BuildIndex([]crawler.Page{
		{URL: "https://x.com/API_Keys/Create-API-Key", Title: "Create API Key"},
	})
	for _, want := range []string{"create", "api", "key", "keys"} {
		if len(idx.Postings[want]) == 0 {
			t.Errorf("expected token %q in postings; postings=%v", want, idx.Postings)
		}
	}
}

func TestIndex_SearchRanksLeafOverHub(t *testing.T) {
	pages := []crawler.Page{
		{URL: "https://x.com/auth", Title: "Auth Overview", Section: "/"},
		{URL: "https://x.com/auth/login", Title: "Login", Section: "auth"},
	}
	idx := manifest.BuildIndex(pages)
	hits := idx.Search("login", 5)
	if len(hits) == 0 {
		t.Fatal("expected hits")
	}
	if pages[hits[0].DocID].Title != "Login" {
		t.Errorf("expected Login first, got %s", pages[hits[0].DocID].Title)
	}
}

func TestIndex_SearchTitleBoost(t *testing.T) {
	pages := []crawler.Page{
		// Mentions 'vector' and 'search' only in URL path.
		{URL: "https://x.com/api/vector/search/internals", Title: "Internals", Section: "api"},
		// Both query tokens in the title — should rank higher despite both matching.
		{URL: "https://x.com/guides/vsg", Title: "Vector Search Guide", Section: "guides"},
	}
	idx := manifest.BuildIndex(pages)
	hits := idx.Search("vector search", 5)
	if len(hits) < 2 {
		t.Fatalf("want 2 hits, got %d", len(hits))
	}
	if pages[hits[0].DocID].Title != "Vector Search Guide" {
		t.Errorf("title-all-tokens boost not applied: top = %q", pages[hits[0].DocID].Title)
	}
}

func TestIndex_SearchRespectsLimit(t *testing.T) {
	pages := samplePages()
	idx := manifest.BuildIndex(pages)
	hits := idx.Search("api", 2)
	if len(hits) > 2 {
		t.Errorf("limit ignored: got %d hits", len(hits))
	}
}

func TestIndex_SearchNoMatchReturnsNil(t *testing.T) {
	idx := manifest.BuildIndex(samplePages())
	if hits := idx.Search("zzzzzzzzz", 5); hits != nil {
		t.Errorf("expected nil for no matches, got %v", hits)
	}
}

func TestIndex_SaveLoadRoundTrip(t *testing.T) {
	withHome(t)
	pages := samplePages()
	want := manifest.BuildIndex(pages)
	if err := manifest.SaveIndex("test", want); err != nil {
		t.Fatal(err)
	}
	got, err := manifest.LoadIndex("test")
	if err != nil {
		t.Fatal(err)
	}
	if got.DocCount != want.DocCount || got.AvgLen != want.AvgLen {
		t.Errorf("round-trip mismatch: want %+v got %+v", want, got)
	}
	if len(got.Postings) != len(want.Postings) {
		t.Errorf("posting count mismatch: want %d got %d", len(want.Postings), len(got.Postings))
	}
	// Sanity: search still works.
	if hits := got.Search("login", 5); len(hits) == 0 {
		t.Error("loaded index returns no hits for known token")
	}
}

func TestIndex_LoadMissingReturnsSentinel(t *testing.T) {
	withHome(t)
	_, err := manifest.LoadIndex("nope")
	if err != manifest.ErrIndexMissing {
		t.Errorf("want ErrIndexMissing, got %v", err)
	}
}

func TestIndex_LoadStaleVersionReturnsSentinel(t *testing.T) {
	withHome(t)
	idx := manifest.BuildIndex(samplePages())
	idx.Version = 0 // pretend this was written by an older pinax
	if err := manifest.SaveIndex("stale", idx); err != nil {
		t.Fatal(err)
	}
	_, err := manifest.LoadIndex("stale")
	if err != manifest.ErrIndexMissing {
		t.Errorf("want ErrIndexMissing for stale version, got %v", err)
	}
}

func TestIndex_SaveBuiltAlongsideManifest(t *testing.T) {
	home := withHome(t)
	m := &manifest.Manifest{Name: "auto", BaseURL: "https://x", Pages: samplePages()}
	if err := manifest.Save(m); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".pinax", "servers")
	if _, err := os.Stat(filepath.Join(dir, "auto.bm25")); err != nil {
		t.Errorf("expected .bm25 file written next to manifest: %v", err)
	}
}

func TestIndex_DeleteRemovesIndex(t *testing.T) {
	home := withHome(t)
	m := &manifest.Manifest{Name: "gone", BaseURL: "https://x", Pages: samplePages()}
	if err := manifest.Save(m); err != nil {
		t.Fatal(err)
	}
	if err := manifest.Delete("gone"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".pinax", "servers", "gone.bm25")); !os.IsNotExist(err) {
		t.Errorf("expected .bm25 deleted: stat err = %v", err)
	}
}

func TestIndex_StopwordsDoNotDominate(t *testing.T) {
	pages := []crawler.Page{
		{URL: "https://x.com/a/the/of/and", Title: "All Stopwords"},
		{URL: "https://x.com/api/users", Title: "User API"},
	}
	idx := manifest.BuildIndex(pages)
	if hits := idx.Search("the of and a", 5); hits != nil {
		t.Errorf("query of pure stopwords should produce nil, got %v", hits)
	}
	hits := idx.Search("user", 5)
	if len(hits) == 0 || !strings.Contains(pages[hits[0].DocID].Title, "User") {
		t.Errorf("expected User result first: %v", hits)
	}
}
