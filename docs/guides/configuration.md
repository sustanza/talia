# Configuration Reference

All CLI flags, environment variables, and `.env` file support.

## CLI Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--whois` | string | — | WHOIS server in `host:port` format. Required for domain checking |
| `--sleep` | duration | `2s` | Delay between sequential WHOIS checks. Ignored in parallel mode |
| `--verbose` | bool | `false` | Include raw WHOIS response in `log` field for all results |
| `--grouped-output` | bool | `false` | Output as `{available:[], unavailable:[]}` instead of array |
| `--output-file` | string | — | Separate file for grouped output (leaves input unchanged) |
| `--suggest` | int | `0` | Number of AI suggestions to generate per request |
| `--suggest-parallel` | int | `1` | Number of concurrent AI suggestion requests |
| `--prompt` | string | — | Natural language prompt to guide AI suggestions |
| `--model` | string | `gpt-5-mini` | AI model name |
| `--api-base` | string | — | Base URL for OpenAI-compatible API |
| `--fresh` | bool | `false` | Don't send existing domains as exclusions to AI |
| `--clean` | bool | `false` | Normalize/deduplicate domains in the file, then exit |
| `--no-verify` | bool | `false` | Skip WHOIS verification after generating suggestions |
| `--merge` | bool | `false` | Merge multiple domain files with deduplication |
| `-o` | string | — | Output file for `--merge` |
| `--export-available` | string | — | Export available domains to a plain text file |
| `--lightspeed` | string | — | Parallel WHOIS: `"max"`, an integer, or empty for sequential |

## Environment Variables

| Variable | Fallback for | Notes |
|---|---|---|
| `TALIA_FILE` | positional arg | Target file path |
| `WHOIS_SERVER` | `--whois` | WHOIS server `host:port` |
| `OPENAI_API_KEY` | — | Required for `--suggest`. No flag equivalent |
| `OPENAI_API_BASE` | `--api-base` | Falls back to `https://api.openai.com/v1` |
| `TALIA_SUGGEST` | `--suggest` | Ignored if file has pending `unverified` domains |
| `TALIA_SUGGEST_PARALLEL` | `--suggest-parallel` | Number of parallel AI requests |
| `TALIA_PROMPT` | `--prompt` | Extra context for AI suggestions |
| `TALIA_MODEL` | `--model` | Only applies when `--model` is at its default value |
| `TALIA_LIGHTSPEED` | `--lightspeed` | Parallel WHOIS worker count |

## Precedence

```
explicit CLI flag  >  shell environment variable  >  .env file
```

### `.env` File

Talia loads a `.env` file from the current working directory at startup.

Example `.env` file:

```
OPENAI_API_KEY=your-api-key
OPENAI_API_BASE=https://generativelanguage.googleapis.com/v1beta/openai
WHOIS_SERVER=whois.verisign-grs.com:43
TALIA_PROMPT=short brandable startup names
TALIA_SUGGEST=10
TALIA_MODEL=gemini-3.1-flash-lite
TALIA_FILE=suggestions.json
```

Rules:

- Does **not** override existing shell environment variables.
- Supports `KEY=VALUE` format (matching quotes — both `"` or both `'` — are stripped from values).
- Lines without `=` are silently skipped.
- No inline comment stripping — `KEY=value # comment` sets the value to `value # comment` (the full right-hand side).
- A variable set to empty string in the shell (`export KEY=""`) counts as "existing" and will not be overwritten.
- Silently ignored if the file doesn't exist.

### Env Var Override Quirks

The env vars for `--model` and `--suggest-parallel` only apply when the flag value equals its hardcoded default. This means explicitly passing the default value on the CLI (e.g., `--model=gpt-5-mini` or `--suggest-parallel=1`) still allows the env var to override it, since the comparison is against the string constant rather than whether the flag was explicitly set. See [Known Issues](../plans/known-issues.md).

### `--sleep` During Auto-Verification

The `--sleep` flag is ignored during the auto-verification step after `--suggest`. Auto-verification uses a hardcoded 100ms delay between WHOIS checks for speed. The `--sleep` value only applies to standalone WHOIS checking runs.

## Related Documentation

- [Development Guide](development.md)
- [Domain Checking](../features/domain-checking.md)
- [AI Suggestions](../features/ai-suggestions.md)
