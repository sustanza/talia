# ADR-001: WHOIS Availability Detection

**Date:** 2025-05-29
**Status:** Accepted

## Context

Talia needs to determine whether a domain name is available for registration. The standard protocol for this is WHOIS, which returns free-text responses with no universal format across registries.

## Decision

Use a single substring match — `"No match for"` — against the raw WHOIS response to determine availability.

- **Available:** response contains `"No match for"` → `NO_MATCH`
- **Taken:** response does not contain the substring → `TAKEN`
- **Error:** TCP connection failure or empty response → `ERROR`

The tool uses raw TCP sockets (`net.Dial`) rather than an HTTP-based WHOIS API. The connection writes `"<domain>\r\n"`, calls `CloseWrite()` to signal EOF, and reads the full response with `io.ReadAll`.

## Alternatives Considered

1. **Registry-specific parsers per TLD** — more accurate but high maintenance cost and scope creep.
2. **RDAP (Registration Data Access Protocol)** — structured JSON responses, but not universally supported across all registrars and adds HTTP dependency.
3. **Third-party WHOIS APIs** — adds external dependency, rate limits, and potential cost.

## Consequences

- **Pro:** Simple, zero-dependency, works reliably with Verisign-style servers (`.com`, `.net`).
- **Pro:** The `WhoisClient` interface makes it trivial to swap implementations later.
- **Con:** The `"No match for"` string is specific to Verisign WHOIS servers. Other registries (e.g., `.io`, `.dev`) use different phrasing and will report all domains as taken.
- **Con:** No retry logic — transient TCP failures are reported as `ERROR` and processing continues.

## Related Documentation

- [Domain Checking](../features/domain-checking.md)
- [Configuration Reference](../guides/configuration.md)
