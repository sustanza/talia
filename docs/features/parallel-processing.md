# Parallel Processing

Concurrent WHOIS checking and AI suggestion requests.

## Parallel WHOIS (`--lightspeed`)

### Modes

| Value | Behavior |
|---|---|
| `"max"` | One goroutine per domain (capped to `len(domains)`) |
| Integer (e.g., `"10"`) | Fixed worker pool of that size |
| Invalid string | Defaults silently to 10 workers |
| Not set | Sequential mode with `--sleep` delay |

### Implementation

- **Worker pool** uses a buffered channel pre-filled with all jobs.
- **Order preservation:** results are written to a pre-indexed slice (`results[job.index]`), so output order matches input regardless of goroutine scheduling.
- **Progress output:** mutex-protected `fmt.Printf` prevents interleaved lines.
- **Statistics:** `atomic.AddInt64` for lock-free counter increments (available, taken, errors, elapsed time).
- **No sleep** between checks in parallel mode.

### Example

```bash
# 20 concurrent workers
talia --whois whois.verisign-grs.com:43 --lightspeed 20 domains.json

# Maximum parallelism (one goroutine per domain)
talia --whois whois.verisign-grs.com:43 --lightspeed max domains.json
```

## Parallel AI Suggestions (`--suggest-parallel`)

### Behavior

- All N goroutines launch simultaneously (no worker pool).
- Each fires an independent HTTP request to the AI API with identical parameters.
- Results are accumulated under a `sync.Mutex` into a shared slice.
- A `sync.WaitGroup` waits for all to complete.

### Failure Handling

- **All requests fail:** exits with code 1 and the first error message.
- **Some fail, some succeed:** prints a warning to stderr, continues with partial results.

### Deduplication

- Parallel requests receive the same exclusion list (snapshot taken before goroutines start), so they may return overlapping suggestions.
- `writeSuggestionsFile()` deduplicates against the full file contents after all requests complete.

### Example

```bash
# 3 parallel requests, 10 suggestions each = up to 30 suggestions
talia --suggest 10 --suggest-parallel 3 --prompt "short tech startup names" domains.json
```

## Related Documentation

- [ADR-003: Parallel Processing Design](../decisions/003-parallel-processing-design.md)
- [Domain Checking](domain-checking.md)
- [AI Suggestions](ai-suggestions.md)
