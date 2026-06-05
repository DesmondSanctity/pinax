package logger_test

import (
	"path/filepath"
	"testing"
	"time"

	"pinax/internal/logger"
)

func openTestStore(t *testing.T) *logger.Store {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test-logger.db")
	s, err := logger.Open(p)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestLogStore_InsertAndList(t *testing.T) {
	s := openTestStore(t)
	if err := s.InsertSync(logger.Entry{
		ServerName:    "convex-docs",
		ToolName:      "get_page",
		Arguments:     `{"url":"https://docs.convex.dev/functions/query"}`,
		ResultPreview: "# Query Functions",
		Status:        "ok",
		DurationMs:    245,
	}); err != nil {
		t.Fatal(err)
	}

	entries := s.List(logger.ListParams{})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ServerName != "convex-docs" {
		t.Errorf("ServerName mismatch: %q", entries[0].ServerName)
	}
	if entries[0].Status != "ok" {
		t.Errorf("Status mismatch: %q", entries[0].Status)
	}
}

func TestLogStore_NewestFirst(t *testing.T) {
	s := openTestStore(t)
	if err := s.InsertSync(logger.Entry{ServerName: "s", ToolName: "list_sections", Status: "ok", DurationMs: 1}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := s.InsertSync(logger.Entry{ServerName: "s", ToolName: "search_pages", Status: "ok", DurationMs: 2}); err != nil {
		t.Fatal(err)
	}

	entries := s.List(logger.ListParams{})
	if entries[0].ToolName != "search_pages" {
		t.Errorf("expected search_pages first (newest), got %q", entries[0].ToolName)
	}
}

func TestLogStore_FilterByServer(t *testing.T) {
	s := openTestStore(t)
	for _, name := range []string{"convex-docs", "supabase-docs"} {
		if err := s.InsertSync(logger.Entry{ServerName: name, ToolName: "get_page", Status: "ok", DurationMs: 1}); err != nil {
			t.Fatal(err)
		}
	}
	entries := s.List(logger.ListParams{Server: "convex-docs"})
	if len(entries) != 1 || entries[0].ServerName != "convex-docs" {
		t.Errorf("filter by server failed: got %v", entries)
	}
}

func TestLogStore_FilterByStatus(t *testing.T) {
	s := openTestStore(t)
	if err := s.InsertSync(logger.Entry{ServerName: "s", ToolName: "get_page", Status: "ok", DurationMs: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertSync(logger.Entry{ServerName: "s", ToolName: "get_page", Status: "error", Error: "HTTP_404", DurationMs: 1}); err != nil {
		t.Fatal(err)
	}

	errs := s.List(logger.ListParams{Status: "error"})
	if len(errs) != 1 || errs[0].Error != "HTTP_404" {
		t.Errorf("filter by status failed: got %+v", errs)
	}
}

func TestLogStore_Limit(t *testing.T) {
	s := openTestStore(t)
	for i := 0; i < 10; i++ {
		if err := s.InsertSync(logger.Entry{ServerName: "s", ToolName: "get_page", Status: "ok", DurationMs: int64(i)}); err != nil {
			t.Fatal(err)
		}
	}
	entries := s.List(logger.ListParams{Limit: 3})
	if len(entries) != 3 {
		t.Errorf("expected 3 entries with limit=3, got %d", len(entries))
	}
}

func TestLogStore_Clear(t *testing.T) {
	s := openTestStore(t)
	if err := s.InsertSync(logger.Entry{ServerName: "s", ToolName: "get_page", Status: "ok", DurationMs: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertSync(logger.Entry{ServerName: "s", ToolName: "get_page", Status: "ok", DurationMs: 2}); err != nil {
		t.Fatal(err)
	}

	deleted, err := s.Clear()
	if err != nil || deleted != 2 {
		t.Errorf("Clear: want (2, nil), got (%d, %v)", deleted, err)
	}
	if len(s.List(logger.ListParams{})) != 0 {
		t.Error("expected empty list after Clear")
	}
}

func TestLogStore_DistinctServers(t *testing.T) {
	s := openTestStore(t)
	for _, name := range []string{"a", "b", "a"} {
		if err := s.InsertSync(logger.Entry{ServerName: name, ToolName: "get_page", Status: "ok", DurationMs: 1}); err != nil {
			t.Fatal(err)
		}
	}
	servers := s.DistinctServers()
	if len(servers) != 2 {
		t.Errorf("expected 2 distinct servers, got %d: %v", len(servers), servers)
	}
}
