# Merge and Export

File merging with deduplication and plain text export of available domains.

## Merge (`--merge`)

Combines two or more domain files into one with deduplication.

### Usage

```bash
# Merge file2 into file1 (file1 is overwritten)
talia --merge domains1.json domains2.json

# Merge into a new output file
talia --merge -o combined.json domains1.json domains2.json domains3.json
```

### Behavior

- Requires at least 2 files, or 1 file with `-o` specified.
- Uses a global `seen` map with **first-write-wins** semantics — once a domain appears in any section, subsequent occurrences in later files are ignored.
- All domains pass through `normalizeDomain()` for validation.
- Reads each file as `ExtendedGroupedData`, accumulates into a single structure.

### Note on Merge Semantics

There are two distinct merge implementations in the codebase:

1. **`mergeFiles()`** (`--merge` flag) — flat `seen` map, first-write-wins, normalizes domains.
2. **`mergeGrouped()`** (`--output-file` with `--grouped-output`) — two maps (available/unavailable), **newest-wins** with bucket switching. A domain moving from taken to available in a newer run will be reclassified. Does not normalize domains.

These have intentionally different semantics for different use cases. Note that `mergeGrouped` operates on `GroupedData` (no `unverified` field), so `unverified` entries are silently dropped when merging via `--output-file`.

## Export Available (`--export-available`)

Writes all available domains from a file to a plain text file (one domain per line).

### Usage

```bash
talia --export-available available.txt domains.json
```

### Behavior

- Reads the input file as `ExtendedGroupedData`.
- Extracts only `data.Available` entries.
- Writes domain names one per line with a trailing newline.
- Order is preserved from the input file.

## Limitations

- `mergeFiles` uses first-write-wins, so file order matters when domains appear in different sections across files.
- `mergeGrouped` (used by `--output-file`) produces non-deterministic JSON ordering due to Go map iteration. See [Known Issues](../plans/known-issues.md).
- `mergeGrouped` operates on `GroupedData` which has no `unverified` field — unverified entries are silently dropped during merge via `--output-file`.

## Related Documentation

- [ADR-004: Output Format Design](../decisions/004-output-format-design.md)
- [Domain Checking](domain-checking.md)
- [File Cleaning](file-cleaning.md) — shares `normalizeDomain()` validation
- [Configuration Reference](../guides/configuration.md)
