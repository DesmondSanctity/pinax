# Security Policy

## Supported versions

Pinax is pre-1.0. Only the `main` branch receives security fixes.

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Use [GitHub's private vulnerability reporting](https://github.com/desmondsanctity/pinax/security/advisories/new)
for this repository and include:

- A description of the vulnerability
- Steps to reproduce (a minimal proof-of-concept is ideal)
- The version or commit SHA you tested against
- Your assessment of impact and any mitigations you can think of

You should receive an acknowledgement within 72 hours. We aim to ship a fix
within 14 days for high-severity issues. Once a fix is released we will
credit you in the release notes unless you prefer to remain anonymous.

## Scope

In scope:

- The `pinax` binary and everything under `internal/`
- The HTTP transport, log viewer, and tool middleware
- The on-disk format of `~/.pinax/` (manifests, page cache, log store)

Out of scope:

- Third-party documentation sites that Pinax crawls
- Misconfigurations of downstream MCP clients
- Vulnerabilities in pinned dependencies that do not affect Pinax's usage of
  them (please still report so we can update)

## Hardening notes for operators

- Pinax makes outbound HTTP requests to whatever URL you ask it to crawl.
  Treat `pinax add` like any other tool that performs URL fetches — do not
  hand it untrusted input on a host with privileged network access.
- The HTTP server (`pinax serve --http`) binds to `localhost` by default and
  has no authentication. Do not expose it on a public interface without a
  reverse proxy that handles auth.
- Cached pages and tool-call logs are written to `~/.pinax/`. If you run on
  a multi-user host, ensure the directory permissions are appropriate.
