// Package middleware contains MCP server middlewares for cross-cutting
// concerns like logging.
package middleware

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"pinax/internal/logger"
)

// previewMax is the maximum number of bytes from a tool result we copy into
// the log for at-a-glance debugging.
const previewMax = 200

// WithLogging wraps a ToolHandlerFunc and records each call to store under
// serverName. Logging failures are swallowed so tool responses are never
// blocked on log I/O.
func WithLogging(store *logger.Store, serverName string, handler server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()
		res, err := handler(ctx, req)
		duration := time.Since(start)

		entry := logger.Entry{
			ServerName: serverName,
			ToolName:   req.Params.Name,
			Arguments:  marshalArgs(req.GetArguments()),
			DurationMs: duration.Milliseconds(),
			CalledAt:   start.UTC(),
		}

		switch {
		case err != nil:
			entry.Status = "error"
			entry.Error = err.Error()
		case res != nil && res.IsError:
			entry.Status = "error"
			entry.Error = previewOf(res)
			entry.ResultPreview = entry.Error
		default:
			entry.Status = "ok"
			entry.ResultPreview = previewOf(res)
		}

		if store != nil {
			_ = store.InsertSync(entry)
		}
		return res, err
	}
}

func marshalArgs(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	b, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func previewOf(res *mcp.CallToolResult) string {
	if res == nil {
		return ""
	}
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return truncate(tc.Text, previewMax)
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
