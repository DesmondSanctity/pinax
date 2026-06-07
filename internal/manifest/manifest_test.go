package manifest_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"pinax/internal/crawler"
	"pinax/internal/manifest"
)

func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestManifest_SaveLoadRoundTrip(t *testing.T) {
	withHome(t)
	m := &manifest.Manifest{
		Name:      "convex-docs",
		BaseURL:   "https://docs.convex.dev",
		Platform:  "docusaurus",
		Source:    "sitemap",
		CrawledAt: time.Now().UTC().Truncate(time.Second),
		Pages: []crawler.Page{
			{URL: "https://docs.convex.dev/intro", Title: "Intro", Section: "/"},
			{URL: "https://docs.convex.dev/functions/query", Title: "Query", Section: "functions"},
		},
	}
	if err := manifest.Save(m); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := manifest.Load("convex-docs")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Name != m.Name || loaded.BaseURL != m.BaseURL || len(loaded.Pages) != 2 {
		t.Errorf("round-trip mismatch: %+v", loaded)
	}
}

func TestManifest_LoadMissingFile(t *testing.T) {
	withHome(t)
	if _, err := manifest.Load("nope"); err == nil {
		t.Error("expected error for missing manifest")
	}
}

func TestManifest_AtomicWrite(t *testing.T) {
	home := withHome(t)
	m := &manifest.Manifest{Name: "atomic", BaseURL: "https://x", Pages: []crawler.Page{{URL: "u"}}}
	if err := manifest.Save(m); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".pinax", "servers")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestManifest_List(t *testing.T) {
	withHome(t)
	for _, n := range []string{"b", "a", "c"} {
		if err := manifest.Save(&manifest.Manifest{Name: n, BaseURL: "https://x"}); err != nil {
			t.Fatal(err)
		}
	}
	names, err := manifest.List()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b", "c"}
	if len(names) != 3 || names[0] != want[0] || names[1] != want[1] || names[2] != want[2] {
		t.Errorf("List sorted: want %v, got %v", want, names)
	}
}

func TestManifest_Delete(t *testing.T) {
	withHome(t)
	if err := manifest.Save(&manifest.Manifest{Name: "gone", BaseURL: "https://x"}); err != nil {
		t.Fatal(err)
	}
	if err := manifest.Delete("gone"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := manifest.Load("gone"); err == nil {
		t.Error("expected Load to fail after Delete")
	}
}

func TestManifest_InvalidName(t *testing.T) {
	withHome(t)
	for _, bad := range []string{"", "../etc", "foo/bar", "foo bar", "foo.bar"} {
		if err := manifest.Save(&manifest.Manifest{Name: bad, BaseURL: "u"}); err == nil {
			t.Errorf("expected error for invalid name %q", bad)
		}
	}
}

func TestManifest_LoadAll(t *testing.T) {
	withHome(t)
	for _, n := range []string{"alpha", "beta"} {
		if err := manifest.Save(&manifest.Manifest{
			Name:    n,
			BaseURL: "https://" + n + ".dev",
			Pages:   []crawler.Page{{URL: "https://" + n + ".dev/x"}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	all, err := manifest.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 manifests, got %d", len(all))
	}
	if all["alpha"].BaseURL != "https://alpha.dev" {
		t.Errorf("alpha BaseURL wrong: %+v", all["alpha"])
	}
}

func TestManifest_LoadAllEmpty(t *testing.T) {
	withHome(t)
	all, err := manifest.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Errorf("want empty map, got %d entries", len(all))
	}
}
