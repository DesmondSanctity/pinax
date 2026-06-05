// Package cache implements a cross-session page content cache backed by SQLite.
//
// Entries have a TTL set at Open time. Reads return (content, hit, err) where
// hit==false signals either a cache miss or an expired entry. The store is
// safe for concurrent use.
package cache

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DefaultTTL is used when Open is called with ttl==0.
const DefaultTTL = 24 * time.Hour

// PageCache stores fetched page content keyed by URL.
type PageCache struct {
	db  *sql.DB
	ttl time.Duration
}

// Open creates or opens a cache at dbPath with the given TTL.
// Pass ttl==0 to use DefaultTTL.
func Open(dbPath string, ttl time.Duration) (*PageCache, error) {
	if dir := filepath.Dir(dbPath); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("page cache: mkdir %s: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS page_cache (
            url         TEXT PRIMARY KEY,
            content     TEXT NOT NULL,
            fetched_at  INTEGER NOT NULL
        );
        CREATE INDEX IF NOT EXISTS page_cache_fetched_idx ON page_cache (fetched_at);
    `); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("page cache: schema: %w", err)
	}
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &PageCache{db: db, ttl: ttl}, nil
}

// Get returns the cached content for url. The boolean indicates a fresh hit;
// a miss or expired entry both return ("", false, nil).
func (c *PageCache) Get(url string) (string, bool, error) {
	var content string
	var fetchedAt int64
	err := c.db.QueryRow(
		`SELECT content, fetched_at FROM page_cache WHERE url = ?`, url,
	).Scan(&content, &fetchedAt)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}

	if time.Since(time.UnixMilli(fetchedAt)) > c.ttl {
		return "", false, nil
	}
	return content, true, nil
}

// Set inserts or overwrites the cache entry for url.
func (c *PageCache) Set(url, content string) error {
	_, err := c.db.Exec(`
        INSERT INTO page_cache (url, content, fetched_at)
        VALUES (?, ?, ?)
        ON CONFLICT(url) DO UPDATE SET
            content    = excluded.content,
            fetched_at = excluded.fetched_at
    `, url, content, time.Now().UnixMilli())
	return err
}

// Clear deletes all cache entries and returns the count deleted.
func (c *PageCache) Clear() (int64, error) {
	res, err := c.db.Exec(`DELETE FROM page_cache`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ClearOlderThan deletes entries whose fetched_at is older than age.
func (c *PageCache) ClearOlderThan(age time.Duration) (int64, error) {
	cutoff := time.Now().Add(-age).UnixMilli()
	res, err := c.db.Exec(`DELETE FROM page_cache WHERE fetched_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Close closes the underlying database.
func (c *PageCache) Close() error { return c.db.Close() }
