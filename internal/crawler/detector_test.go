package crawler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pinax/internal/crawler"
)

func TestDetectPlatform_Docusaurus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><script src="/__docusaurus/debug"></script></head></html>`))
	}))
	defer srv.Close()

	result := crawler.DetectPlatform(srv.URL)
	if result.Platform != crawler.PlatformDocusaurus {
		t.Errorf("expected Docusaurus, got %s", result.Platform)
	}
	if !result.Supported {
		t.Error("Docusaurus should be supported")
	}
}

func TestDetectPlatform_MintlifyUnsupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><link href="/mintlify-worker.js"/></head></html>`))
	}))
	defer srv.Close()

	result := crawler.DetectPlatform(srv.URL)
	if result.Platform != crawler.PlatformMintlify {
		t.Errorf("expected Mintlify, got %s", result.Platform)
	}
	if result.Supported {
		t.Error("Mintlify should NOT be supported — JS rendering required")
	}
}

func TestDetectPlatform_UnknownIsSupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>docs</body></html>`))
	}))
	defer srv.Close()

	result := crawler.DetectPlatform(srv.URL)
	if !result.Supported {
		t.Error("Unknown platform should be attempted")
	}
}
