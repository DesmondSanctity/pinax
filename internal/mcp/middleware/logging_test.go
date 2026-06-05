package middleware_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"pinax/internal/logger"
	"pinax/internal/mcp/middleware"
)

func openStore(t *testing.T) *logger.Store {
	t.Helper()
	s, err := logger.Open(filepath.Join(t.TempDir(), "log.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func testRequest(name string, args map[string]any) mcp.CallToolRequest {
	var r mcp.CallToolRequest
	r.Params.Name = name
	r.Params.Arguments = args
	return r
}

func successHandler(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("hello world"), nil
}

func errorHandler(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return nil, errors.New("kaboom")
}

func toolErrorHandler(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("bad input"), nil
}

func TestLogging_RecordsOkCall(t *testing.T) {
	s := openStore(t)
	h := middleware.WithLogging(s, "convex-docs", successHandler)
	if _, err := h(context.Background(), testRequest("get_page", map[string]any{"url": "https://x"})); err != nil {
		t.Fatal(err)
	}
	entries := s.List(logger.ListParams{})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.ServerName != "convex-docs" || e.ToolName != "get_page" || e.Status != "ok" {
		t.Errorf("bad entry: %+v", e)
	}
	if e.ResultPreview != "hello world" {
		t.Errorf("preview mismatch: %q", e.ResultPreview)
	}
	if e.Arguments == "{}" {
		t.Errorf("arguments not captured: %q", e.Arguments)
	}
}

func TestLogging_RecordsGoError(t *testing.T) {
	s := openStore(t)
	h := middleware.WithLogging(s, "srv", errorHandler)
	if _, err := h(context.Background(), testRequest("get_page", nil)); err == nil {
		t.Error("expected error to propagate")
	}
	entries := s.List(logger.ListParams{})
	if len(entries) != 1 || entries[0].Status != "error" || entries[0].Error != "kaboom" {
		t.Errorf("error not logged correctly: %+v", entries)
	}
}

func TestLogging_RecordsToolError(t *testing.T) {
	s := openStore(t)
	h := middleware.WithLogging(s, "srv", toolErrorHandler)
	if _, err := h(context.Background(), testRequest("get_page", nil)); err != nil {
		t.Fatal(err)
	}
	entries := s.List(logger.ListParams{})
	if len(entries) != 1 || entries[0].Status != "error" || entries[0].Error != "bad input" {
		t.Errorf("tool error not logged correctly: %+v", entries)
	}
}

func TestLogging_NilStoreIsSafe(t *testing.T) {
	h := middleware.WithLogging(nil, "srv", successHandler)
	if _, err := h(context.Background(), testRequest("x", nil)); err != nil {
		t.Errorf("nil store should not produce handler error: %v", err)
	}
}

func TestLogging_TruncatesPreview(t *testing.T) {
	s := openStore(t)
	long := make([]byte, 500)
	for i := range long {
		long[i] = 'a'
	}
	h := middleware.WithLogging(s, "srv", func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(string(long)), nil
	})
	if _, err := h(context.Background(), testRequest("x", nil)); err != nil {
		t.Fatal(err)
	}
	entries := s.List(logger.ListParams{})
	if len(entries[0].ResultPreview) > 205 {
		t.Errorf("preview not truncated: len=%d", len(entries[0].ResultPreview))
	}
}
