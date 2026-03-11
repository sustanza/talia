# Testing Guide

Test architecture, mocking strategy, and CI integration.

## Running Tests

```bash
# Standard
go test -v

# With race detection and coverage
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

## Test Files

| File | Scope |
|---|---|
| `main_test.go` | Integration tests for all CLI paths |
| `whois_test.go` | Unit tests for WHOIS client via `fakeWhoisClient` |
| `suggestions_test.go` | Unit and integration tests for AI suggestion pipeline |
| `cmd/talia/main_test.go` | Tests that `main()` exits non-zero with no args |

All library tests are in the `talia` package (white-box), giving access to unexported types and functions.

## Mocking Strategy

### WHOIS Mocking

Two approaches:

1. **`WhoisClient` interface** — `fakeWhoisClient` returns hardcoded responses for pure unit tests of availability logic.
2. **In-process TCP listeners** — `net.Listen("tcp", "127.0.0.1:0")` with goroutines serving predictable responses for integration tests that exercise the full TCP path.

### HTTP Mocking (AI API)

- `httptest.NewServer` provides a local HTTP server with controlled responses.
- The `httpDoer` interface and package-level `testHTTPClient`/`testBaseURL` vars are the injection points.
- Integration tests (`TestRunCLISuggest`, etc.) set these vars directly to route requests through the test server.

## Test Isolation

`TestMain` (in `main_test.go`) runs before all tests and:

- Sets `skipEnvFile = true` to prevent `.env` file loading during tests.
- Unsets `OPENAI_API_KEY` and `OPENAI_API_BASE` to prevent real API calls.

Individual tests that modify env vars use `defer os.Unsetenv(...)` for cleanup.

## Output Capture

The `captureOutput` helper (in `main_test.go`) uses `os.Pipe()` to redirect `os.Stdout` and `os.Stderr`, allowing test assertions on terminal output without consuming the actual terminal.

## Parallelism in Tests

- Pure unit tests are marked `t.Parallel()`.
- Integration tests that touch shared package-level vars (`testHTTPClient`, `testBaseURL`) are **not** parallel.
- CI runs `go test -race ./...` — the parallel WHOIS checker is designed to be race-detector clean (atomic counters, result-by-index writes, mutex for printing).

## Key Patterns

- **Flag isolation:** `RunCLI` uses `flag.NewFlagSet` (not `flag.CommandLine`), so parallel tests don't share flag state.
- **Temp files:** Tests create temporary files for input/output and clean up via `defer os.Remove(...)`.
- **Exit code testing:** `cmd/talia/main_test.go` overrides the `exitFunc` variable to capture exit codes without actually calling `os.Exit`.

## Related Documentation

- [Development Guide](development.md)
- [Domain Checking](../features/domain-checking.md)
- [AI Suggestions](../features/ai-suggestions.md)
