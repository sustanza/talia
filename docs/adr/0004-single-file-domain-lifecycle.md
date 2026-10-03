# One JSON file carries the whole domain lifecycle

A single domain file holds every domain in its current state (unverified, available, or unavailable) and is rewritten in place on each run, so suggest → check → re-check needs no other storage. Talia accepts either a flat array or a grouped object and detects which by trying to parse the file, with no format flag.

## Considered Options

- **SQLite or another database**: overkill for a CLI that processes one list at a time, and opaque to `jq`.
- **YAML/TOML**: no advantage over JSON, which Go and shell tooling handle natively.
