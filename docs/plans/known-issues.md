# Known Issues

**Last updated:** 2026-03-10

## Open Issues

### Non-deterministic merge output ordering

**Severity:** Low
**Component:** `grouped.go` — `mergeGrouped()`

`mergeGrouped()` uses Go map iteration to build the output arrays, producing non-deterministic JSON ordering on each run. This causes noisy diffs if the output file is version-controlled.

**Workaround:** Sort the output externally with `jq` if stable ordering is needed.

---

### Duplicate `.env` loaders

**Severity:** Low
**Component:** `cmd/talia/main.go` vs `env.go`

Two `.env` loading implementations exist:
- `cmd/talia/main.go` (lines 12-26): inline loader that **always overwrites** existing env vars.
- `env.go` / `LoadEnvFile()`: more complete loader with quote stripping that **does not override** existing vars.

Both run at startup. The `main.go` loader runs first, so its overwrite behavior takes precedence for any vars set in `.env`. Additionally, the `main.go` loader does not strip quotes from values (uses raw `strings.Cut` + `strings.TrimSpace`), while `LoadEnvFile` does strip matching quotes.

---

### `--lightspeed` silent default

**Severity:** Low
**Component:** `cli.go`

Invalid `--lightspeed` values (non-integer, non-`"max"` strings) silently default to 10 workers with no warning. Users may not realize their input was ignored.

---

### `--model` env var override quirk

**Severity:** Low
**Component:** `cli.go`

`TALIA_MODEL` overrides `--model` when the flag value equals the hardcoded default (`gpt-5-mini`), even if the user explicitly passed `--model=gpt-5-mini`. The comparison checks the string value, not whether the flag was explicitly set.

---

### WHOIS detection limited to Verisign servers

**Severity:** Medium
**Component:** `whois.go`

The `"No match for"` availability check only works with Verisign-style WHOIS servers (`.com`, `.net`). Other registries use different response formats and will silently report all domains as taken.

**Mitigation:** Documented in README and [ADR-001](../decisions/001-whois-availability-detection.md). The `WhoisClient` interface allows swapping in a more sophisticated implementation.

---

### ANSI color codes with no TTY detection

**Severity:** Low
**Component:** `progress.go`

Progress output uses ANSI escape codes unconditionally. If stdout is piped or redirected to a file, raw escape codes will appear in the output. There is no `isatty` check to disable colors.

## Related Documentation

- [ADR-001: WHOIS Availability Detection](../decisions/001-whois-availability-detection.md)
- [ADR-004: Output Format Design](../decisions/004-output-format-design.md)
- [Configuration Reference](../guides/configuration.md)
