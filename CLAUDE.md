# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
# Build the binary
go build -o talia

# Run tests
go test -v

# Run tests with race detection and coverage
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# Lint (uses Go tool directive - auto-downloads on first run)
go tool golangci-lint run
```

## Architecture

Talia is a CLI tool for checking domain availability via WHOIS servers and generating domain suggestions via OpenAI.

**Core modules (root package `talia`):**

- **cli.go** - Entry point with flag parsing. `RunCLI()` routes to either `RunCLIDomainArray()` (plain array format) or `RunCLIGroupedInput()` (grouped format with unverified domains)
- **whois.go** - WHOIS client using TCP sockets. `WhoisClient` interface enables testing with mock implementations. Availability detected by "No match for" substring
- **types.go** - Data structures: `DomainRecord` (array format), `GroupedDomain`/`GroupedData`/`ExtendedGroupedData` (grouped format)
- **grouped.go** - JSON merge logic for grouped output format with deduplication
- **suggestions.go** - OpenAI-compatible API integration using structured output for domain generation. Supports OpenAI, Gemini, and other compatible APIs via `--api-base` flag or `OPENAI_API_BASE` env var (requires `OPENAI_API_KEY`)

**cmd/talia/main.go** - Thin wrapper that calls `talia.RunCLI()`

## Commit Convention

Uses Conventional Commits (enforced by commitlint in CI). Allowed types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `perf`, `style`, `revert`.

Example: `feat: add new flag for TLD filtering`
