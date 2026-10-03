# Availability is a substring match on raw WHOIS

A domain is available when the raw WHOIS response, read over a plain TCP connection to a user-specified server, contains `"No match for"`; anything else is `TAKEN`, and a failed or empty response is `ERROR`. This is crude but dependency-free and correct for Verisign's `.com` server, which is the only registry Talia targets (see [ADR-0003](0003-com-only.md)).

## Considered Options

- **RDAP**: structured JSON, but uneven registrar support and an HTTP dependency.
- **Per-registry response parsers**: more accurate, but a maintenance burden for TLDs Talia doesn't support.
- **Third-party WHOIS APIs**: rate limits, cost, and an external dependency.

## Consequences

Pointed at any non-Verisign server, every domain reports as taken, silently. There is no retry: a transient TCP failure becomes `ERROR` and the run continues. The `WhoisClient` interface is the seam for a smarter implementation.
