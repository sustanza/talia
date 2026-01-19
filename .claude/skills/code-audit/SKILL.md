---
name: code-audit
description: Audits code against Go best practices, Google style guide, idiomatic patterns, and dependency decisions. Run after completing features to ensure quality.
allowed-tools: Read, Glob, Grep, WebFetch, WebSearch
---

# Go Code Audit

Run this skill after completing a feature or significant code changes to ensure compliance with Go best practices and project conventions.

## Audit Checklist

### Google Go Style Guide

Reference: https://google.github.io/styleguide/go/

- [ ] Package names are short, lowercase, singular (no underscores)
- [ ] Exported names are clear without package prefix redundancy
- [ ] Comments on exported items start with the item name
- [ ] No stuttering in names (e.g., `http.HTTPServer` is bad)
- [ ] Receiver names are short (1-2 letters), consistent across methods
- [ ] Error variable names use `err` or descriptive `fooErr` pattern

### Effective Go & Idiomatic Patterns

Reference: https://go.dev/doc/effective_go

- [ ] Errors returned, not panicked (panic only for truly unrecoverable)
- [ ] Error wrapping uses `fmt.Errorf("context: %w", err)` pattern
- [ ] Errors checked immediately after call (no deferred checks)
- [ ] `defer` used for cleanup (close files, unlock mutexes)
- [ ] Named return values only when they improve documentation
- [ ] Naked returns avoided except in very short functions
- [ ] Interface segregation (small interfaces, often single method)
- [ ] Accept interfaces, return structs
- [ ] Zero values are useful (no unnecessary constructors)

### Go Proverbs

Reference: https://go-proverbs.github.io/

- [ ] Clear is better than clever
- [ ] A little copying is better than a little dependency
- [ ] Don't just check errors, handle them gracefully
- [ ] The bigger the interface, the weaker the abstraction
- [ ] Make the zero value useful

### Concurrency Patterns

Reference: https://go.dev/doc/effective_go#concurrency

- [ ] Goroutines have clear ownership and lifecycle
- [ ] Channels used for communication, mutexes for state
- [ ] `context.Context` passed as first parameter
- [ ] Context cancellation respected in long operations
- [ ] No goroutine leaks (ensure goroutines can exit)
- [ ] `sync.WaitGroup` or channels for goroutine coordination
- [ ] Race conditions addressed (`go test -race` passes)

### Dependencies: Build vs Import

- [ ] Standard library preferred over third-party packages
- [ ] Dependencies justified (not just convenience)
- [ ] Simple utilities built in-project rather than imported
- [ ] No dependency for single-use functions
- [ ] Dependency versions pinned in go.mod
- [ ] Indirect dependencies minimized
- [ ] No deprecated or unmaintained dependencies

### Testing Standards

Reference: https://go.dev/doc/tutorial/add-a-test

- [ ] Table-driven tests for multiple cases
- [ ] Test names use `TestXxx_condition` pattern
- [ ] `t.Helper()` called in test helper functions
- [ ] `t.Parallel()` used where tests are independent
- [ ] Subtests used for grouped test cases (`t.Run`)
- [ ] No test pollution (tests clean up after themselves)
- [ ] Mocks/interfaces used for external dependencies
- [ ] Edge cases and error paths tested

### Project Structure

Reference: https://go.dev/doc/modules/layout

- [ ] Flat structure preferred (avoid deep nesting)
- [ ] `internal/` used for private packages
- [ ] `cmd/` used for multiple executables
- [ ] No `src/` directory (Go convention)
- [ ] Package by feature, not by layer

### Code Organization

- [ ] Functions ordered: exported first, then private
- [ ] Related functions grouped together
- [ ] File size reasonable (split large files by concern)
- [ ] No circular dependencies between packages
- [ ] `init()` used sparingly and only when necessary

## Project-Specific Patterns

Check `CLAUDE.md` for project conventions:

### From CLAUDE.md

- [ ] Follows commit convention (conventional commits)
- [ ] Uses project's build commands correctly
- [ ] Linter passes (`go tool golangci-lint run`)
- [ ] Tests pass with race detection

### Code Style

- [ ] No third-party dependencies without justification
- [ ] Conventional commits format used
- [ ] README.md updated if features changed

## How to Run Audit

1. Identify changed/added files in the recent work
2. Read each file and check against the checklist above
3. Run `go vet` and linter to catch static issues
4. Reference Go documentation for pattern questions
5. Report findings with specific file:line references
6. Suggest fixes for any violations

## Audit Output Format

```
## Audit Results: [Feature Name]

### Passed
- [x] Item that passed

### Issues Found
- [ ] **file.go:42** - Issue description
  - Suggestion: How to fix

### Recommendations
- Optional improvements (not violations)
```
