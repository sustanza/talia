# ADR-004: Output Format Design

**Date:** 2025-05-29
**Status:** Accepted

## Context

Talia needs to persist domain checking results. The tool supports multiple workflows — simple batch checking, AI-driven suggestion pipelines, and file merging — each with different output needs.

## Decision

Support three output formats, with auto-detection on input:

### 1. Array Format (default)

```json
[
  {"domain": "example.com", "available": true, "reason": "NO_MATCH"},
  {"domain": "taken.com", "available": false, "reason": "TAKEN"}
]
```

- Input file is updated in place.
- Fields `available`, `reason`, and `log` are `omitempty`.
- `log` only populated when `--verbose` is set or `reason == "ERROR"`.

### 2. Grouped Format (`--grouped-output`)

```json
{
  "available": [{"domain": "example.com", "reason": "NO_MATCH"}],
  "unavailable": [{"domain": "taken.com", "reason": "TAKEN"}]
}
```

- Uses `GroupedDomain` type (always includes `reason`).
- With `--output-file`, leaves the input file untouched and writes/merges to the specified output.

### 3. Extended Grouped Format (suggestion workflow)

```json
{
  "available": [...],
  "unavailable": [...],
  "unverified": [{"domain": "new-suggestion.com"}]
}
```

- The `unverified` array holds AI-generated suggestions pending WHOIS verification.
- After verification, domains move to `available`/`unavailable` and `unverified` is set to `nil` (omitted from JSON via `omitempty`).
- A single file represents all workflow states without format changes.

### Input auto-detection

`RunCLI` tries `json.Unmarshal` into `[]DomainRecord` first (array), then `ExtendedGroupedData` (object). The JSON structure itself determines the code path — no flag needed.

### Plain text

Used only for `--export-available` output and as an alternative `--clean` input. One domain per line, no JSON wrapper.

## Alternatives Considered

1. **Single format only** — Simpler but forces all users into one workflow. The grouped format is essential for the suggestion pipeline.
2. **YAML or TOML** — JSON is simpler, has native Go support, and works directly with `jq` and other CLI tools.
3. **Database (SQLite)** — Over-engineered for a CLI tool that processes files.

## Consequences

- **Pro:** `ExtendedGroupedData` with `omitempty` elegantly represents the full suggestion→verify lifecycle in one file.
- **Pro:** Auto-detection means users never need to specify the format — the tool just works.
- **Con:** Two merge implementations exist (`mergeGrouped` in `grouped.go` and `mergeFiles` in `suggestions.go`) with different semantics. See [Known Issues](../plans/known-issues.md).
- **Con:** `mergeGrouped` uses map iteration, producing non-deterministic JSON key ordering on each run.

## Related Documentation

- [Domain Checking](../features/domain-checking.md)
- [AI Suggestions](../features/ai-suggestions.md)
- [Merge and Export](../features/merge-and-export.md)
- [File Cleaning](../features/file-cleaning.md)
