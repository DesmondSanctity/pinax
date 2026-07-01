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

// New constructs an MCPServer with all pinax tools registered.
//
// `manifests` may contain one entry (legacy `pinax serve <name>`) or many
// (unified `pinax serve`). When `reload` is non-nil it is invoked at the
// start of routing tool calls so new manifests appear without restart.
//
// `displayName` becomes the MCP server name reported to clients. Pass "pinax"
// for unified mode, "pinax/<name>" for single-name mode.
//
// If `logStore` is non-nil every tool call is recorded using `displayName`.
func New(
	manifests map[string]*manifest.Manifest,
	c *cache.PageCache,
	logStore *logger.Store,
	displayName string,
	reload func() (map[string]*manifest.Manifest, error),
) *mcpsrv.MCPServer {
	s := mcpsrv.NewMCPServer(
		displayName,
		Version,
		mcpsrv.WithToolCapabilities(false),
	)
	deps := tools.New(manifests, c)
	deps.Reload = reload
	deps.Register(s, func(h mcpsrv.ToolHandlerFunc) mcpsrv.ToolHandlerFunc {
		return middleware.WithLogging(logStore, displayName, h)
	})
	// Renderer registry is populated lazily by tools.Deps on first fetch of
	// a page whose manifest declares a renderer. That keeps hot-added
	// manifests working without a server restart.
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
