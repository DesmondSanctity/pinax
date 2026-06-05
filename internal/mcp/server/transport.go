package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	mcpsrv "github.com/mark3labs/mcp-go/server"

	"pinax/internal/logger"
)

// HTTPOptions configures the HTTP transport.
type HTTPOptions struct {
	Addr    string // e.g. ":8080"
	LogPath string // optional path to the log viewer UI (default "/")
}

// ListenAndServeHTTP runs the streamable-HTTP MCP endpoint at POST /mcp,
// the SSE transport at /sse + /message, a /health endpoint, and the log
// viewer + API. Blocks until ctx is cancelled or the server errors.
func ListenAndServeHTTP(ctx context.Context, s *mcpsrv.MCPServer, store *logger.Store, opts HTTPOptions) error {
	if opts.Addr == "" {
		opts.Addr = ":8080"
	}
	if opts.LogPath == "" {
		opts.LogPath = "/"
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", NewStreamableHTTP(s))
	sse := NewSSE(s)
	mux.Handle("/sse", sse.SSEHandler())
	mux.Handle("/message", sse.MessageHandler())

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "ok")
	})

	if store != nil {
		mux.Handle("/api/", store.API())
		mux.Handle(opts.LogPath, logger.UI())
	}

	srv := &http.Server{
		Addr:              opts.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
