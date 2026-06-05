package cache_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"pinax/internal/cache"
)

func openTestCache(t *testing.T) *cache.PageCache {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test-cache.db")
	c, err := cache.Open(p, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to open cache: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestPageCache_GetMissOnEmpty(t *testing.T) {
	c := openTestCache(t)
	content, hit, err := c.Get("https://docs.example.com/page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hit {
		t.Error("expected cache miss on empty store, got hit")
	}
	if content != "" {
		t.Errorf("expected empty content on miss, got %q", content)
	}
}

func TestPageCache_SetAndGet(t *testing.T) {
	c := openTestCache(t)
	url := "https://docs.example.com/functions/query"
	want := "# Query Functions\n\nContent here."

	if err := c.Set(url, want); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, hit, err := c.Get(url)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !hit {
		t.Error("expected cache hit after Set, got miss")
	}
	if got != want {
		t.Errorf("content mismatch: got %q, want %q", got, want)
	}
}

func TestPageCache_TTLExpiry(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ttl-cache.db")
	c, err := cache.Open(p, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { c.Close() })

	url := "https://docs.example.com/page"
	if err := c.Set(url, "content"); err != nil {
		t.Fatalf("set: %v", err)
	}

	if _, hit, _ := c.Get(url); !hit {
		t.Error("expected cache hit within TTL")
	}

	time.Sleep(100 * time.Millisecond)
	if _, hit, _ := c.Get(url); hit {
		t.Error("expected cache miss after TTL expiry, got hit")
	}
}

func TestPageCache_SetOverwrites(t *testing.T) {
	c := openTestCache(t)
	url := "https://docs.example.com/page"
	if err := c.Set(url, "original content"); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(url, "updated content"); err != nil {
		t.Fatal(err)
	}
	got, hit, _ := c.Get(url)
	if !hit {
		t.Fatal("expected hit after overwrite")
	}
	if got != "updated content" {
		t.Errorf("expected overwritten content, got %q", got)
	}
}

func TestPageCache_Clear(t *testing.T) {
	c := openTestCache(t)
	for _, u := range []string{"a", "b", "c"} {
		if err := c.Set("https://docs.example.com/"+u, "content "+u); err != nil {
			t.Fatal(err)
		}
	}
	deleted, err := c.Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
	if deleted != 3 {
		t.Errorf("expected 3 deleted, got %d", deleted)
	}
	if _, hit, _ := c.Get("https://docs.example.com/a"); hit {
		t.Error("expected miss after Clear")
	}
}

func TestPageCache_ClearOlderThan(t *testing.T) {
	p := filepath.Join(t.TempDir(), "older.db")
	c, err := cache.Open(p, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })

	if err := c.Set("https://docs.example.com/old", "old content"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := c.Set("https://docs.example.com/new", "new content"); err != nil {
		t.Fatal(err)
	}

	deleted, err := c.ClearOlderThan(25 * time.Millisecond)
	if err != nil {
		t.Fatalf("ClearOlderThan failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	if _, hit, _ := c.Get("https://docs.example.com/old"); hit {
		t.Error("expected old entry to be deleted")
	}
	if _, hit, _ := c.Get("https://docs.example.com/new"); !hit {
		t.Error("expected new entry to survive")
	}
}

func TestPageCache_MultipleURLsIndependent(t *testing.T) {
	c := openTestCache(t)
	urls := []string{
		"https://docs.example.com/a",
		"https://docs.example.com/b",
		"https://docs.example.com/c",
	}
	for _, u := range urls {
		if err := c.Set(u, "content for "+u); err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range urls {
		got, hit, _ := c.Get(u)
		if !hit {
			t.Errorf("expected hit for %s", u)
		}
		if got != "content for "+u {
			t.Errorf("content mismatch for %s: got %q", u, got)
		}
	}
}

// Sanity check: file is actually created and reusable.
func TestPageCache_PersistsAcrossOpen(t *testing.T) {
	p := filepath.Join(t.TempDir(), "persist.db")
	c1, err := cache.Open(p, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := c1.Set("https://x/y", "hello"); err != nil {
		t.Fatal(err)
	}
	c1.Close()

	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected db file: %v", err)
	}

	c2, err := cache.Open(p, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c2.Close() })

	got, hit, _ := c2.Get("https://x/y")
	if !hit || got != "hello" {
		t.Errorf("expected persisted value 'hello' hit, got %q hit=%v", got, hit)
	}
}
