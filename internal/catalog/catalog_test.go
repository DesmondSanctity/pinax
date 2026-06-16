package catalog_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"pinax/internal/catalog"
)

func TestLoad_EmbeddedHasEntries(t *testing.T) {
	c := catalog.Load()
	if c == nil || len(c.Entries) == 0 {
		t.Fatalf("expected non-empty embedded catalog")
	}
	if c.Version == "" {
		t.Errorf("missing version")
	}
	for k, e := range c.Entries {
		if e.URL == "" {
			t.Errorf("entry %q missing url", k)
		}
	}
}

func TestLookup_CaseInsensitive(t *testing.T) {
	c := catalog.Load()
	a, ok := c.Lookup("stripe")
	if !ok {
		t.Fatal("expected stripe entry")
	}
	b, ok := c.Lookup("STRIPE")
	if !ok || b.URL != a.URL {
		t.Errorf("case-insensitive lookup mismatch: %+v vs %+v", a, b)
	}
	if _, ok := c.Lookup("definitely-not-a-real-name"); ok {
		t.Error("expected miss")
	}
}

func TestRefresh_WritesCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"test-1","entries":{"foo":{"displayName":"Foo","url":"https://foo.example/docs"}}}`))
	}))
	defer srv.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(catalog.EnvURL, srv.URL)

	got, err := catalog.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "test-1" {
		t.Errorf("version: %s", got.Version)
	}
	if _, err := os.Stat(filepath.Join(home, ".pinax", "catalog.json")); err != nil {
		t.Errorf("cache not written: %v", err)
	}

	loaded := catalog.Load()
	if loaded.Version != "test-1" {
		t.Errorf("Load did not pick up refreshed cache, got %s", loaded.Version)
	}
}

func TestRefresh_RejectsBadPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"entries":{"bad":{"displayName":"no url"}}}`))
	}))
	defer srv.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(catalog.EnvURL, srv.URL)

	if _, err := catalog.Refresh(context.Background()); err == nil {
		t.Fatal("expected validation error")
	}
	if _, err := os.Stat(filepath.Join(home, ".pinax", "catalog.json")); err == nil {
		t.Error("cache should not have been written for invalid payload")
	}
}
