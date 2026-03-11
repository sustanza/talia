# Domain Checking

WHOIS-based domain availability verification.

## Overview

Talia checks domain availability by connecting to a WHOIS server over raw TCP and interpreting the response. Domains are classified as available (`NO_MATCH`), taken (`TAKEN`), or errored (`ERROR`).

## How It Works

1. Opens a TCP connection to the configured `--whois` server (e.g., `whois.verisign-grs.com:43`).
2. Sends `"<domain>\r\n"` and half-closes the write side (`CloseWrite`) to signal EOF.
3. Reads the full response with `io.ReadAll`.
4. Checks for the substring `"No match for"` in the response:
   - **Found** → domain is available (`NO_MATCH`)
   - **Not found** → domain is taken (`TAKEN`)
   - **Connection error or empty response** → `ERROR`

## Input Formats

The tool auto-detects the input format:

- **Array format** — `[]DomainRecord` (JSON array of objects with `domain` field)
- **Extended grouped format** — `ExtendedGroupedData` (JSON object with `available`, `unavailable`, `unverified` arrays)

See [Output Format Design](../decisions/004-output-format-design.md) for format details.

## Sequential vs Parallel

- **Sequential** (default): checks one domain at a time with `--sleep` delay (default `2s`) between requests.
- **Parallel** (`--lightspeed`): uses a worker pool for concurrent checks. See [Parallel Processing](parallel-processing.md).

## Error Handling

- Errors do not abort the run. A failed domain gets `available=false`, `reason=ERROR`, and the error message in the `log` field.
- The exit code is `0` as long as the file write succeeds.
- The `log` field is populated for errors regardless of `--verbose`. For successful checks, `log` only appears when `--verbose` is set.

## Progress Output

Each domain check prints a line to stdout:

```
[1/50] example.com ✓ available
[2/50] taken.com ✗ taken
[3/50] broken.com ⚠ error
```

In parallel mode, output lines are mutex-protected to prevent interleaving. A summary with counts and elapsed time is printed after all checks complete.

## Limitations

- The `"No match for"` detection string is specific to Verisign-style WHOIS servers (`.com`, `.net`). Other registries use different phrasing and will report all domains as taken.
- No TLD routing — a single WHOIS server is used for all domains in the file.
- No retry logic for transient TCP failures.

## Related Documentation

- [ADR-001: WHOIS Availability Detection](../decisions/001-whois-availability-detection.md)
- [Parallel Processing](parallel-processing.md)
- [Configuration Reference](../guides/configuration.md)
