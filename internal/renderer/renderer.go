// Package renderer routes page fetches through a JS-capable service when a
// docs site is a JavaScript SPA that serves empty HTML shells to plain HTTP
// clients. Today it ships a single implementation, JinaRenderer, targeting
// https://r.jina.ai.
package renderer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"pinax/internal/buildinfo"
)

// Names identify which renderer produced a page. Stored verbatim in the
// manifest so the runtime knows which impl to route through.
const (
	NameJina = "jina"
	NameOff  = ""
)

// ErrNoAPIKey is returned when a renderer that needs an API key doesn't
// find one in either the passed Options or the environment.
var ErrNoAPIKey = errors.New("renderer: JINA_API_KEY not set — get a free key at https://jina.ai/reader")

// Renderer converts a canonical page URL into extracted Markdown by
// asking a JS-capable service to render and serialise the page.
type Renderer interface {
	Name() string
	Fetch(ctx context.Context, pageURL string) (string, error)
}

// Options tunes JinaRenderer. Zero-value fields get defaults.
type Options struct {
	APIKey      string        // Bearer token; falls back to $JINA_API_KEY.
	Endpoint    string        // Base reader URL; default https://r.jina.ai/.
	Concurrency int           // Max in-flight requests; default 8.
	RPM         int           // Requests-per-minute cap; default 400 (headroom under the free-tier 500).
	Timeout     time.Duration // Per-request timeout; default 60s.
	HTTPClient  *http.Client
	UserAgent   string
}

// DefaultOptions returns production defaults for JinaRenderer.
func DefaultOptions() Options {
	return Options{
		Endpoint:    "https://r.jina.ai/",
		Concurrency: 8,
		RPM:         400,
		Timeout:     60 * time.Second,
	}
}

// JinaRenderer calls the Jina Reader HTTP API. Requests are throttled by a
// concurrency semaphore and a request-per-minute spacer so a batch of
// pages stays well inside the account's rate cap.
type JinaRenderer struct {
	opts   Options
	sem    chan struct{}
	spacer *spacer
	client *http.Client
}

// NewJina builds a JinaRenderer. Returns ErrNoAPIKey when no key is
// configured. Nil options == DefaultOptions.
func NewJina(opts Options) (*JinaRenderer, error) {
	def := DefaultOptions()
	if opts.Endpoint == "" {
		opts.Endpoint = def.Endpoint
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = def.Concurrency
	}
	if opts.RPM <= 0 {
		opts.RPM = def.RPM
	}
	if opts.Timeout <= 0 {
		opts.Timeout = def.Timeout
	}
	if opts.APIKey == "" {
		opts.APIKey = os.Getenv("JINA_API_KEY")
	}
	if opts.APIKey == "" {
		return nil, ErrNoAPIKey
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: opts.Timeout}
	}
	if opts.UserAgent == "" {
		opts.UserAgent = buildinfo.UserAgent()
	}

	interval := time.Minute / time.Duration(opts.RPM)
	return &JinaRenderer{
		opts:   opts,
		sem:    make(chan struct{}, opts.Concurrency),
		spacer: &spacer{interval: interval},
		client: opts.HTTPClient,
	}, nil
}

// Name implements Renderer.
func (j *JinaRenderer) Name() string { return NameJina }

// Fetch requests Markdown for pageURL, respecting concurrency + RPM caps.
// Callers must not encode pageURL — the raw URL is appended to the Jina
// endpoint exactly as documented at https://r.jina.ai.
func (j *JinaRenderer) Fetch(ctx context.Context, pageURL string) (string, error) {
	pageURL = strings.TrimSpace(pageURL)
	if pageURL == "" {
		return "", fmt.Errorf("renderer: empty url")
	}
	select {
	case j.sem <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-j.sem }()

	if err := j.spacer.wait(ctx); err != nil {
		return "", err
	}

	endpoint := strings.TrimRight(j.opts.Endpoint, "/") + "/" + pageURL
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+j.opts.APIKey)
	req.Header.Set("Accept", "text/markdown")
	req.Header.Set("User-Agent", j.opts.UserAgent)

	resp, err := j.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("renderer: %s: %w", pageURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("renderer: %s → HTTP %d: %s", pageURL, resp.StatusCode, truncateForError(string(body)))
	}
	return string(body), nil
}

// spacer is a global request-spacer: every call to wait() reserves the
// next slot at `max(now, lastReserved+interval)`. Combined with a
// concurrency semaphore this delivers a smooth `RPM` request cadence
// regardless of per-request latency variance.
type spacer struct {
	mu       sync.Mutex
	interval time.Duration
	last     time.Time
}

func (s *spacer) wait(ctx context.Context) error {
	s.mu.Lock()
	now := time.Now()
	scheduled := s.last.Add(s.interval)
	if scheduled.Before(now) {
		scheduled = now
	}
	s.last = scheduled
	delay := time.Until(scheduled)
	s.mu.Unlock()
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func truncateForError(s string) string {
	const max = 200
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
