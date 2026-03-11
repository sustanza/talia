# File Cleaning

Domain normalization, validation, and deduplication for JSON and plain text files.

## Overview

The `--clean` flag normalizes and removes invalid domains from a file, then exits. It auto-detects whether the file is JSON or plain text.

## Format Auto-Detection

The file content is tested with `json.Valid()`:
- **Valid JSON** → `cleanSuggestionsFile()` — processes `ExtendedGroupedData`
- **Not valid JSON** → `cleanTextFile()` — processes line-by-line plain text

## JSON Cleaning (`cleanSuggestionsFile`)

1. Parses the file as `ExtendedGroupedData`.
2. Runs every domain through `normalizeDomain()`.
3. Removes domains that fail validation.
4. Deduplicates across all three sections using a `seen` map. Processing order: available → unavailable → unverified. A domain appearing in both `available` and `unverified` keeps the `available` entry.
5. Writes back the cleaned structure.

## Plain Text Cleaning (`cleanTextFile`)

1. Reads the file line by line.
2. Skips blank lines and lines starting with `#`.
3. Runs each line through `normalizeDomain()`.
4. Deduplicates (first occurrence wins).
5. Writes back as newline-joined list with a trailing newline.
6. Order is preserved from input (minus removed entries). Output is not sorted.

## Validation Rules (`normalizeDomain`)

| Rule | Example |
|---|---|
| Lowercase and trim whitespace | `" Example.COM "` → `"example.com"` |
| Strip repeated `.com` suffixes | `"foo.com.com.com"` → `"foo.com"` |
| Collapse double dots | `"foo..com"` → `"foo.com"` |
| Must end with `.com` | `"foo.io"` → rejected |
| Exactly two dot-separated parts | `"sub.foo.com"` → rejected |
| Label matches `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$` | `"foo-bar.com"` → valid; `"-foo.com"` → rejected |
| Single-character labels allowed | `"a.com"` → valid |

## Output

Prints a count header, then lists each removed domain with an indented `-` prefix:

```
Removed 2 invalid domains:
  - invalid-domain
  - another.bad.domain
Cleaned domains.json
```

If nothing was removed:
```
No invalid domains found.
```

## Usage

```bash
# Clean a JSON domain file
talia --clean domains.json

# Clean a plain text domain file
talia --clean domains.txt
```

## Limitations

- No inline comment stripping for `.env`-style `# comment` at end of lines in plain text files — lines must start with `#` to be treated as comments.
- Deduplication priority is fixed: available > unavailable > unverified. There is no way to customize this order.

## Related Documentation

- [ADR-004: Output Format Design](../decisions/004-output-format-design.md) — defines the `ExtendedGroupedData` structure that `cleanSuggestionsFile` processes
- [AI Suggestions](ai-suggestions.md) — suggestions pass through the same `normalizeDomain` validation
- [Configuration Reference](../guides/configuration.md)
