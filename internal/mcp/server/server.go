// Package server wires the pinax MCP server: registers tools and exposes
// helpers for stdio/SSE/HTTP transports.
package server

import (
	"context"
	"net/http"

	mcpsrv "github.com/mark3labs/mcp-go/server"

	"pinax/internal/cache"
	"pinax/internal/logger"
	"pinax/internal/manifest"
	"pinax/internal/mcp/middleware"
	"pinax/internal/mcp/tools"
)

// Version is the pinax protocol version reported to MCP clients.
const Version = "1.0.0"

// New constructs an MCPServer with all four pinax tools registered. If
// logStore is non-nil every tool call is recorded as serverName.
func New(m *manifest.Manifest, c *cache.PageCache, logStore *logger.Store, serverName string) *mcpsrv.MCPServer {
	s := mcpsrv.NewMCPServer(
		"pinax/"+serverName,
		Version,
		mcpsrv.WithToolCapabilities(false),
	)
	deps := tools.New(m, c)
	deps.Register(s, func(h mcpsrv.ToolHandlerFunc) mcpsrv.ToolHandlerFunc {
		return middleware.WithLogging(logStore, serverName, h)
	})
	return s
}

// ServeStdio runs the MCP server over stdio (Claude Desktop / Code default).
func ServeStdio(ctx context.Context, s *mcpsrv.MCPServer) error {
	_ = ctx
	return mcpsrv.ServeStdio(s)
}

// NewStreamableHTTP returns an HTTP handler for the Streamable HTTP transport,
// configured stateless so each request is self-contained.
func NewStreamableHTTP(s *mcpsrv.MCPServer) http.Handler {
	return mcpsrv.NewStreamableHTTPServer(s, mcpsrv.WithStateLess(true))
}

// NewSSE returns the SSE transport (for legacy clients).
func NewSSE(s *mcpsrv.MCPServer) *mcpsrv.SSEServer {
	return mcpsrv.NewSSEServer(s)
}
