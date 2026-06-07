// Package manifest stores and loads Pinax server manifests under ~/.pinax/servers.
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"pinax/internal/crawler"
)

// Manifest describes a single pinax server: the source URL list and metadata.
type Manifest struct {
	Name      string         `json:"name"`
	BaseURL   string         `json:"baseUrl"`
	Platform  string         `json:"platform"`
	Source    string         `json:"source"`
	CrawledAt time.Time      `json:"crawledAt"`
	Pages     []crawler.Page `json:"pages"`
}

// Dir returns the directory where manifests are stored, creating it if needed.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".pinax", "servers")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// Path returns the on-disk path for a given server name.
func Path(name string) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".json"), nil
}

// Save writes the manifest atomically.
func Save(m *Manifest) error {
	if err := validateName(m.Name); err != nil {
		return err
	}
	p, err := Path(m.Name)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".manifest-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, p); err != nil {
		return err
	}
	// Rebuild the search index alongside the manifest. Failure here is logged
	// to stderr but doesn't fail the save — a missing index causes
	// search_pages to fall back to its legacy ranker.
	if err := SaveIndex(m.Name, BuildIndex(m.Pages)); err != nil {
		fmt.Fprintf(os.Stderr, "pinax: warning: failed to write search index for %q: %v\n", m.Name, err)
	}
	return nil
}

// Load reads a manifest by name.
func Load(name string) (*Manifest, error) {
	p, err := Path(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("manifest %q not found — run 'pinax add' first", name)
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest %q: %w", name, err)
	}
	return &m, nil
}

// List returns the names of all saved manifests, sorted.
func List() ([]string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(names)
	return names, nil
}

// LoadAll loads every manifest on disk, keyed by name. Manifests that fail to
// parse are skipped silently — they will surface via `pinax list` and `pinax
// doctor`. Safe to call on every tool invocation; manifest reads are tiny.
func LoadAll() (map[string]*Manifest, error) {
	names, err := List()
	if err != nil {
		return nil, err
	}
	out := make(map[string]*Manifest, len(names))
	for _, n := range names {
		m, err := Load(n)
		if err != nil {
			continue
		}
		out[n] = m
	}
	return out, nil
}

// Delete removes a saved manifest.
func Delete(name string) error {
	p, err := Path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("manifest %q not found", name)
		}
		return err
	}
	// Best-effort: remove the companion index file too. Missing is fine.
	_ = DeleteIndex(name)
	return nil
}

func validateName(name string) error {
	if name == "" {
		return errors.New("manifest name must not be empty")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return fmt.Errorf("manifest name %q contains invalid character %q (allowed: a-z A-Z 0-9 - _)", name, r)
		}
	}
	return nil
}

// FromCrawlResult builds a Manifest from a CrawlResult.
func FromCrawlResult(name string, r *crawler.CrawlResult) *Manifest {
	return &Manifest{
		Name:      name,
		BaseURL:   r.BaseURL,
		Platform:  r.Platform,
		Source:    r.Source,
		CrawledAt: r.CrawledAt,
		Pages:     r.Pages,
	}
}
