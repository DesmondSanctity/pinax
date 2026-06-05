# Contributing to Pinax

Thanks for your interest in contributing. This document covers what you need
to know to get a patch merged.

## Code of Conduct

This project adheres to the [Contributor Covenant](CODE_OF_CONDUCT.md). By
participating you are expected to uphold it. Report unacceptable behaviour by
opening a private discussion on the repository or messaging a maintainer
directly.

## Ground rules

- **Open an issue first** for anything beyond a small fix. A 5-line discussion
  on the issue tracker saves a 500-line PR going in the wrong direction.
- **One logical change per PR.** Refactors, formatting sweeps, and feature
  work go in separate PRs.
- **No new dependencies without justification.** Pinax is deliberately small;
  every dependency is a long-term maintenance commitment.
- **Keep the scope tight.** Pinax is intentionally a small local CLI. For
  anything beyond a bug fix or a focused improvement, open an issue first so
  we can confirm it fits before you spend time on a PR.

## Development setup

Requires Go 1.25 or newer. macOS, Linux, and WSL are supported.

```sh
git clone https://github.com/desmondsanctity/pinax.git
cd pinax
make build
make test
```

### Useful commands

| Command                 | What it does                                   |
| ----------------------- | ---------------------------------------------- |
| `make build`            | Build the `pinax` binary to `./bin/pinax`      |
| `make test`             | Unit tests with `-race`, 120 s timeout         |
| `make test-integration` | Network-touching integration tests (build tag) |
| `make vet`              | `go vet ./...`                                 |
| `make fmt`              | `gofmt -s -w .`                                |
| `make tidy`             | `go mod tidy`                                  |

### Running it end-to-end locally

```sh
./bin/pinax add https://docs.convex.dev
./bin/pinax serve convex-docs --http --port 8423
# Open http://localhost:8423/ for the log viewer
```

## Coding conventions

- **Format with `gofmt -s`.** CI rejects unformatted code.
- **Errors are values.** Wrap with `fmt.Errorf("operation: %w", err)`; do not
  swallow.
- **Comments are sparing.** Prefer self-documenting names. Add a one-liner
  when intent is non-obvious; do not narrate code in block comments.
- **No `panic` in library code.** `cmd/pinax` may exit; `internal/*` returns
  errors.
- **Validate at boundaries only.** Don't re-validate args between internal
  functions; trust the type system.
- **Public surface stays small.** If a symbol doesn't need to be exported,
  don't export it.
- **No global state.** Pass dependencies through constructors (`tools.New`,
  `cache.Open`, `manifest.Store`, etc.).
- **Tests live next to the code** in `_test.go` files, in the `_test` package
  where practical, to enforce the public-API contract.

## Tests

Every behaviour change ships with a test. We use the standard library
`testing` package; no external test framework.

- **Unit tests** (`make test`) must pass with `-race` and complete in
  under 120 s. Use `httptest.NewServer` for HTTP behaviour; do not hit the
  network.
- **Integration tests** (`make test-integration`) live behind the
  `//go:build integration` tag and may make real network calls. Keep them
  deterministic — pin to URLs that change slowly and assert on stable
  invariants (page count > N, section names contain X).
- **Bug fixes ship with a regression test** that fails before the fix and
  passes after. Reference the issue in the test name when relevant.

Run a single package:

```sh
go test ./internal/mcp/tools -run TestSearchPages -count=1 -v
```

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(crawler): filter sitemap URLs by base path
fix(tools): tokenize natural-language queries
docs: explain the four-tool design
test(cache): cover sub-second TTLs
chore: bump golang.org/x/net to v0.31.0
```

Scopes match the top-level `internal/*` package or `cmd/pinax`.

## Pull request checklist

Before opening a PR, make sure:

- [ ] `make fmt vet test` passes locally
- [ ] New behaviour has a test; bug fixes have a regression test
- [ ] No new dependencies, or justification in the PR description
- [ ] No changes to `go.mod` other than what your code requires
- [ ] `README.md` updated if you changed user-visible behaviour
- [ ] Commit messages follow Conventional Commits
- [ ] PR description explains the _why_, not just the _what_

## Reporting bugs

Open an issue with:

1. Pinax version (`pinax --version`)
2. Go version (`go version`) and OS
3. The exact command and the docs URL involved
4. Expected vs actual behaviour
5. Relevant lines from `~/.pinax/calls.db` (visible in the log UI) or the
   server's stderr

## Security

Do not report vulnerabilities via public issues. Use
[GitHub Security Advisories](https://github.com/desmondsanctity/pinax/security/advisories/new).
See [SECURITY.md](SECURITY.md) for the full policy.

## License

By contributing, you agree that your contributions will be licensed under the
[MIT License](LICENSE).
