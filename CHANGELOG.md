# Changelog

All notable changes to Pinax are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial release — Go CLI that turns any documentation site into a local
  MCP server.
- Discovery via `llms.txt` probe, sitemap, or bounded BFS crawl.
- Four MCP tools: `list_sections`, `search_pages`, `get_section_pages`,
  `get_page`.
- Natural-language token search (`search_pages` with substring AND-matching
  across tokens, fuzzy fallback for typos).
- Stdio, Streamable HTTP, and legacy SSE transports.
- Persistent SQLite page cache and in-memory session cache.
- Built-in dark-themed log viewer at the HTTP root.
- `pinax --version` with commit/build-date stamping.
- Per-subcommand help via `pinax help <command>`.

### Security

- Pinned `golang.org/x/net` to `v0.55.0` (closes called HTML-parser advisories).
- Minimum Go version raised to `1.25` (toolchain `go1.25.11`) so stdlib
  vulnerability scans come back clean. `govulncheck ./...` reports no findings.
- Tightened per-user storage directories (`~/.pinax/`, cache, manifests, logs)
  to mode `0o700`.

[Unreleased]: https://github.com/desmondsanctity/pinax/compare/HEAD
