# CLAUDE.md

Talia is a CLI tool for checking `.com` domain availability via WHOIS and generating domain suggestions via OpenAI-compatible APIs. All source lives in the root `talia` package with a thin binary wrapper at `cmd/talia/main.go`. Uses Conventional Commits (`feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `perf`, `style`, `revert`).

## Quick Commands

```bash
go build -o talia              # build
go test -v                     # test
go test -race -coverprofile=coverage.out ./...  # test with race + coverage
go tool golangci-lint run      # lint
```

## Documentation

Full docs live in [`docs/`](docs/README.md):

### Decisions (architecture rationale)
- [001 — WHOIS Availability Detection](docs/decisions/001-whois-availability-detection.md)
- [002 — AI Suggestion Architecture](docs/decisions/002-ai-suggestion-architecture.md)
- [003 — Parallel Processing Design](docs/decisions/003-parallel-processing-design.md)
- [004 — Output Format Design](docs/decisions/004-output-format-design.md)

### Features (behavior specs)
- [Domain Checking](docs/features/domain-checking.md) — WHOIS-based availability verification
- [AI Suggestions](docs/features/ai-suggestions.md) — AI-powered domain name generation
- [File Cleaning](docs/features/file-cleaning.md) — domain normalization and deduplication
- [Merge and Export](docs/features/merge-and-export.md) — file merging and plain text export
- [Parallel Processing](docs/features/parallel-processing.md) — concurrent WHOIS and suggestion requests

### Guides (development and operations)
- [Development Guide](docs/guides/development.md) — building, running, and contributing
- [Configuration Reference](docs/guides/configuration.md) — all flags, env vars, and `.env` support
- [Testing Guide](docs/guides/testing.md) — test architecture, mocking, and CI

### Plans (open risks)
- [Known Issues](docs/plans/known-issues.md) — quirks and limitations
