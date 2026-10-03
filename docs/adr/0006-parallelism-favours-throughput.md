# Parallelism favours throughput over politeness and uniqueness

Lightspeed checking runs a worker pool with no sleep and no rate limiting; the user controls load through the worker count, and results keep input order. Parallel suggestion requests all fire at once with the same exclusion list, so they can return overlapping suggestions, which are deduplicated when written to the file.

## Considered Options

- **Rate-limited parallel checks**: deferred; the worker count is the throttle.
- **Feeding earlier suggestion results into later requests as exclusions**: would serialize the requests and defeat the point.

## Consequences

`--lightspeed max` opens one connection per domain at once, which can get you rate-limited or banned by the WHOIS server.
