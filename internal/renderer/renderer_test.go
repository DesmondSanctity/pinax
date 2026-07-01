package renderer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewJina_RequiresAPIKey(t *testing.T) {
	t.Setenv("JINA_API_KEY", "")
	if _, err := NewJina(Options{}); !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("expected ErrNoAPIKey, got %v", err)
	}
}

func TestNewJina_UsesEnvVar(t *testing.T) {
	t.Setenv("JINA_API_KEY", "env-key")
	r, err := NewJina(Options{})
	if err != nil {
		t.Fatalf("NewJina: %v", err)
	}
	if r.opts.APIKey != "env-key" {
		t.Fatalf("expected APIKey from env, got %q", r.opts.APIKey)
	}
}

func TestJinaFetch_SendsAuthAndAppendsURL(t *testing.T) {
	var seenAuth, seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		seenAuth = req.Header.Get("Authorization")
		seenPath = req.URL.Path + "?" + req.URL.RawQuery
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = io.WriteString(w, "# rendered\n\nreal prose")
	}))
	defer srv.Close()

	r, err := NewJina(Options{APIKey: "test-key", Endpoint: srv.URL + "/"})
	if err != nil {
		t.Fatalf("NewJina: %v", err)
	}
	got, err := r.Fetch(context.Background(), "https://example.com/docs/foo?bar=1")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(got, "real prose") {
		t.Fatalf("body missing: %q", got)
	}
	if seenAuth != "Bearer test-key" {
		t.Fatalf("auth header = %q", seenAuth)
	}
	if !strings.Contains(seenPath, "https://example.com/docs/foo") {
		t.Fatalf("upstream path = %q, want target URL appended", seenPath)
	}
}

func TestJinaFetch_PropagatesUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "quota exceeded")
	}))
	defer srv.Close()

	r, err := NewJina(Options{APIKey: "k", Endpoint: srv.URL + "/"})
	if err != nil {
		t.Fatalf("NewJina: %v", err)
	}
	_, err = r.Fetch(context.Background(), "https://example.com/")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 403") || !strings.Contains(err.Error(), "quota") {
		t.Fatalf("error missing context: %v", err)
	}
}

func TestJinaFetch_RespectsCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		<-req.Context().Done()
	}))
	defer srv.Close()

	r, err := NewJina(Options{
		APIKey:      "k",
		Endpoint:    srv.URL + "/",
		Concurrency: 1,
		Timeout:     100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewJina: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := r.Fetch(ctx, "https://example.com/"); err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestJinaFetch_ConcurrencyAndSpacing(t *testing.T) {
	var (
		inflight    int32
		maxInflight int32
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cur := atomic.AddInt32(&inflight, 1)
		for {
			mx := atomic.LoadInt32(&maxInflight)
			if cur <= mx || atomic.CompareAndSwapInt32(&maxInflight, mx, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inflight, -1)
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	r, err := NewJina(Options{
		APIKey:      "k",
		Endpoint:    srv.URL + "/",
		Concurrency: 2,
		RPM:         60000,
	})
	if err != nil {
		t.Fatalf("NewJina: %v", err)
	}

	// Fire 8 concurrent fetches; only 2 should run at once.
	done := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func(i int) {
			_, err := r.Fetch(context.Background(), "https://example.com/p"+string(rune('0'+i)))
			done <- err
		}(i)
	}
	for i := 0; i < 8; i++ {
		if err := <-done; err != nil {
			t.Fatalf("fetch %d: %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&maxInflight); got > 2 {
		t.Fatalf("concurrency ceiling violated: saw %d in-flight, want <=2", got)
	}
}

func TestSpacerEnforcesInterval(t *testing.T) {
	s := &spacer{interval: 15 * time.Millisecond}
	start := time.Now()
	for i := 0; i < 4; i++ {
		if err := s.wait(context.Background()); err != nil {
			t.Fatalf("wait: %v", err)
		}
	}
	elapsed := time.Since(start)
	// 4 reservations at 15ms spacing → first is free, next 3 space out.
	if elapsed < 45*time.Millisecond {
		t.Fatalf("spacer too fast: %v (want >= 45ms)", elapsed)
	}
}
