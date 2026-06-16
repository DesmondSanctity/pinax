// Package catalog ships a curated map of well-known docs sites so users can
// run `pinax add stripe` instead of remembering canonical URLs. The embedded
// catalog can be overridden by a JSON file at ~/.pinax/catalog.json, which
// `pinax catalog refresh` fetches from a configurable URL.
package catalog

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"pinax/internal/buildinfo"
)

//go:embed catalog.json
var embedded []byte

// DefaultURL is the canonical over-the-air catalog source. Override with the
// PINAX_CATALOG_URL environment variable.
const DefaultURL = "https://raw.githubusercontent.com/DesmondSanctity/pinax/main/internal/catalog/catalog.json"

// EnvURL is the env var that overrides DefaultURL.
const EnvURL = "PINAX_CATALOG_URL"

// Entry is one curated docs site.
type Entry struct {
	DisplayName string   `json:"displayName"`
	URL         string   `json:"url"`
	LLMsTxt     string   `json:"llmsTxt,omitempty"`
	Platform    string   `json:"platform,omitempty"`
	Excludes    []string `json:"excludes,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// Catalog is the on-disk + embedded shape.
type Catalog struct {
	Version string           `json:"version"`
	Entries map[string]Entry `json:"entries"`
}

// Load returns the catalog, preferring the cached override at
// ~/.pinax/catalog.json when present and falling back to the binary's
// embedded copy. A corrupt override is ignored (with a warning to stderr)
// rather than fatal — first-run UX must always work.
func Load() *Catalog {
	if path, err := cachePath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			c, err := parse(data)
			if err == nil {
				return c
			}
			fmt.Fprintf(os.Stderr, "pinax: ignoring corrupt catalog cache (%s): %v\n", path, err)
		}
	}
	c, err := parse(embedded)
	if err != nil {
		// Embedded catalog being unparseable is a build-time bug.
		panic(fmt.Sprintf("embedded catalog: %v", err))
	}
	return c
}

// Lookup returns the entry for name (case-insensitive). The bool reports
// whether a match was found.
func (c *Catalog) Lookup(name string) (Entry, bool) {
	if c == nil {
		return Entry{}, false
	}
	e, ok := c.Entries[strings.ToLower(strings.TrimSpace(name))]
	return e, ok
}

// Names returns every catalog key sorted alphabetically.
func (c *Catalog) Names() []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.Entries))
	for k := range c.Entries {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Refresh fetches the catalog from PINAX_CATALOG_URL (or DefaultURL) and,
// if it parses, atomically replaces the on-disk cache. Returns the parsed
// catalog so callers can show the version that landed.
func Refresh(ctx context.Context) (*Catalog, error) {
	url := os.Getenv(EnvURL)
	if url == "" {
		url = DefaultURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", buildinfo.UserAgent())
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("catalog refresh: %s %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, err
	}
	c, err := parse(body)
	if err != nil {
		return nil, fmt.Errorf("catalog payload at %s is invalid: %w", url, err)
	}
	path, err := cachePath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return nil, err
	}
	return c, nil
}

func parse(data []byte) (*Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.Entries == nil {
		return nil, fmt.Errorf("missing entries map")
	}
	for k, e := range c.Entries {
		if e.URL == "" {
			return nil, fmt.Errorf("entry %q missing url", k)
		}
	}
	return &c, nil
}

func cachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".pinax", "catalog.json"), nil
}
