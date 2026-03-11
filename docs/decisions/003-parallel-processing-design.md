# ADR-003: Parallel Processing Design

**Date:** 2026-01-20
**Status:** Accepted

## Context

Sequential WHOIS checking with a 2-second sleep between requests is reliable but slow for large domain lists. Similarly, generating many AI suggestions benefits from concurrent API calls.

## Decision

Implement two independent parallelism mechanisms:

### Parallel WHOIS (`--lightspeed`)

- Uses a **worker pool** with a buffered channel of jobs.
- Workers drain the channel and write results to a **pre-indexed slice** (`results[job.index]`), preserving input order.
- Three modes:
  - `"max"` → one goroutine per domain (capped to `len(domains)`)
  - Integer string (e.g., `"10"`) → fixed worker count
  - Invalid string → defaults silently to 10 workers
- Sleep between checks is skipped entirely in parallel mode.
- Progress output is mutex-protected to prevent interleaved lines.
- Statistics use `atomic.AddInt64` for lock-free counter increments.

### Parallel Suggestions (`--suggest-parallel`)

- All N goroutines launch simultaneously (no worker pool — each fires one HTTP request).
- Results are accumulated under a `sync.Mutex` into a shared `allResults` slice.
- Partial failure is tolerated: if some requests succeed and others fail, a warning is printed and partial results are used.
- Deduplication across parallel responses happens in `writeSuggestionsFile()` after all goroutines complete.

## Alternatives Considered

1. **Rate-limited parallel** — Add configurable rate limiting to parallel WHOIS. Deferred; users can control concurrency via the worker count.
2. **Streaming results** — Write results as they arrive instead of collecting all first. Adds complexity for minimal user benefit since the file is written atomically.
3. **Context-aware parallel suggestions** — Pass results from earlier parallel requests as exclusions to later ones. Would serialize requests and defeat the purpose.

## Consequences

- **Pro:** Pre-indexed result slice guarantees deterministic output order regardless of goroutine scheduling.
- **Pro:** Atomic counters and mutex-protected printing are race-detector clean.
- **Pro:** Partial failure handling for suggestions avoids losing all results due to one bad request.
- **Con:** Invalid `--lightspeed` values default silently to 10 — no warning is emitted.
- **Con:** `"max"` mode with thousands of domains will open that many TCP connections simultaneously, which may trigger WHOIS server rate limiting.

## Related Documentation

- [Parallel Processing](../features/parallel-processing.md)
- [Domain Checking](../features/domain-checking.md)
- [AI Suggestions](../features/ai-suggestions.md)
