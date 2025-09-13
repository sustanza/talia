# Repository Guidelines

## Project Structure & Module Organization
- Go module: `github.com/sustanza/talia` (top-level package contains core logic).
- CLI entrypoint: `cmd/talia/main.go`; tests alongside as `main_test.go`.
- Source files: top level (`cli.go`, `whois.go`, `grouped.go`, `suggestions.go`, `types.go`).
- Tests: standard `_test.go` files in root and `cmd/talia/`.
- Build output: `dist/`; temporary/work files: `tmp/`; coverage: `coverage.out`.

## Build, Test, and Development Commands
- `make init` — install required tools and download deps.
- `make build` — build `talia` binary to `dist/talia`.
- `make run` — build then print CLI help.
- `make test` / `make test-verbose` — run unit tests (race + cover flags enabled).
- `make test-coverage` — write `coverage.out` and `coverage.html`.
- `make lint` — run `golangci-lint` with repo config; `make fmt` formats code.
- `make check` — fmt, vet, lint, security; `make ci` — local CI pipeline.
- Useful: `go test ./...`, `go tool golangci-lint run`, `make docker-build`.

## Coding Style & Naming Conventions
- Go formatting is mandatory: run `make fmt` before PRs.
- Follow https://google.github.io/styleguide/go/.
- Lint is enforced by `golangci-lint` (see `.golangci.yml`); fix or justify findings.

## Testing Guidelines
- Use table-driven tests in `*_test.go` (standard `testing`). Add edge/error cases.
- Mock externals via interfaces: `WhoisClient` (WHOIS) and `httpDoer`/`suggestionHTTPClient` (OpenAI HTTP).
- Run with race detector before pushing (`go test -race ./...`). Aim for ≥90% total coverage; don’t regress.
- Integration tests (if any) should use `-tags=integration` and run via `make test-integration`.

## Design & Compatibility
- Interface-first design: define or extend interfaces for new externals before implementation.
- Maintain backward compatibility for both input formats: array of domain records and grouped JSON (with optional `unverified`). If changing schemas, update types in `types.go` and migration logic.
- When writing grouped results, prefer `WriteGroupedFile` to merge with existing data.
- If adding flags or outputs, update usage in `cmd/talia/main.go`, `README.MD`, and Makefile examples.

## Commit & Pull Request Guidelines
- Use Conventional Commits; enforced by commitlint. Allowed types include: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `perf`, `style`, `revert`.
- Release Please is used for versioning: `fix:` → patch, `feat:` → minor, `feat!` or `BREAKING CHANGE:` → major.
- PRs must: describe the change and rationale, link issues, include tests, pass `make ci`, and update docs/usage when flags or behavior change.

## Security & Configuration Tips
- Never commit secrets. For domain suggestions, set `OPENAI_API_KEY` in your environment (not in code).
- OpenAI base URL/model are package variables; keep them overrideable for tests.

## Tools
- Use GH CLI for all git operations, unless otherwise specified.