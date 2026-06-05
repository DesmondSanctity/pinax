// Package logger persists MCP tool-call events to SQLite and exposes a
// listing API used by both the HTTP log API and the built-in viewer UI.
package logger

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Entry is a single recorded tool call.
type Entry struct {
	ID            string    `json:"id"`
	ServerName    string    `json:"serverName"`
	ToolName      string    `json:"toolName"`
	Arguments     string    `json:"arguments"`
	ResultPreview string    `json:"resultPreview"`
	Status        string    `json:"status"`
	Error         string    `json:"error,omitempty"`
	DurationMs    int64     `json:"durationMs"`
	CalledAt      time.Time `json:"calledAt"`
}

// ListParams filters and limits a list query.
type ListParams struct {
	Limit  int
	Server string
	Tool   string
	Status string
}

// Store is a SQLite-backed log writer. Safe for concurrent use.
type Store struct {
	db *sql.DB

	asyncOnce sync.Once
	queue     chan Entry
	wg        sync.WaitGroup
}

// Open opens or creates a log database at dbPath.
func Open(dbPath string) (*Store, error) {
	if dir := filepath.Dir(dbPath); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("log store: mkdir %s: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS tool_calls (
            id              TEXT PRIMARY KEY,
            server_name     TEXT NOT NULL,
            tool_name       TEXT NOT NULL,
            arguments       TEXT NOT NULL DEFAULT '{}',
            result_preview  TEXT NOT NULL DEFAULT '',
            status          TEXT NOT NULL CHECK (status IN ('ok','error')),
            error           TEXT,
            duration_ms     INTEGER NOT NULL,
            called_at       INTEGER NOT NULL
        );
        CREATE INDEX IF NOT EXISTS tool_calls_called_at_idx ON tool_calls (called_at DESC);
        CREATE INDEX IF NOT EXISTS tool_calls_server_idx    ON tool_calls (server_name);
        CREATE INDEX IF NOT EXISTS tool_calls_tool_idx      ON tool_calls (tool_name);
    `); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("log store: schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close shuts down the async writer (if any) and the underlying database.
func (s *Store) Close() error {
	if s.queue != nil {
		close(s.queue)
		s.wg.Wait()
	}
	return s.db.Close()
}

// InsertSync writes one entry directly. Used by tests and for guaranteed-flush
// semantics. Returns nil on success.
func (s *Store) InsertSync(e Entry) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.CalledAt.IsZero() {
		e.CalledAt = time.Now().UTC()
	}
	if e.Status == "" {
		e.Status = "ok"
	}
	_, err := s.db.Exec(`
        INSERT INTO tool_calls
            (id, server_name, tool_name, arguments, result_preview, status, error, duration_ms, called_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `, e.ID, e.ServerName, e.ToolName, e.Arguments, e.ResultPreview, e.Status, nullIfEmpty(e.Error), e.DurationMs, e.CalledAt.UnixMilli())
	return err
}

// Insert queues an entry for async write. The MCP response is never blocked
// on the database write; failures are surfaced via Errors() (not implemented
// — log writes are best-effort by design).
func (s *Store) Insert(e Entry) {
	s.asyncOnce.Do(s.startWriter)
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.CalledAt.IsZero() {
		e.CalledAt = time.Now().UTC()
	}
	select {
	case s.queue <- e:
	default:
		// Queue full — drop rather than block the caller.
	}
}

func (s *Store) startWriter() {
	s.queue = make(chan Entry, 256)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for e := range s.queue {
			_ = s.InsertSync(e)
		}
	}()
}

// List returns entries matching params, newest first.
func (s *Store) List(p ListParams) []Entry {
	if p.Limit <= 0 {
		p.Limit = 500
	}
	var (
		conds []string
		args  []any
	)
	if p.Server != "" {
		conds = append(conds, "server_name = ?")
		args = append(args, p.Server)
	}
	if p.Tool != "" {
		conds = append(conds, "tool_name = ?")
		args = append(args, p.Tool)
	}
	if p.Status != "" {
		conds = append(conds, "status = ?")
		args = append(args, p.Status)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, p.Limit)

	// where is built from a fixed allowlist of conditions; user-supplied
	// values bind through ? placeholders, so this concat is safe.
	// gosec G202 is excluded for this file in .golangci.yml.
	query := `
        SELECT id, server_name, tool_name, arguments, result_preview,
               status, COALESCE(error, ''), duration_ms, called_at
        FROM tool_calls` + where + `
        ORDER BY called_at DESC
        LIMIT ?`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		var e Entry
		var calledAtMs int64
		if err := rows.Scan(&e.ID, &e.ServerName, &e.ToolName, &e.Arguments,
			&e.ResultPreview, &e.Status, &e.Error, &e.DurationMs, &calledAtMs); err != nil {
			continue
		}
		e.CalledAt = time.UnixMilli(calledAtMs).UTC()
		out = append(out, e)
	}
	return out
}

// Clear removes all entries and returns the number deleted.
func (s *Store) Clear() (int64, error) {
	res, err := s.db.Exec(`DELETE FROM tool_calls`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DistinctServers returns the unique server_name values in the store.
func (s *Store) DistinctServers() []string {
	rows, err := s.db.Query(`SELECT DISTINCT server_name FROM tool_calls ORDER BY server_name`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			out = append(out, name)
		}
	}
	return out
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
