# AI Suggestions

AI-powered domain name generation via OpenAI-compatible APIs.

## Overview

Talia generates domain name suggestions by calling a large language model through the OpenAI chat completions API. It uses tool calling (function calling) to enforce structured JSON output, and supports any OpenAI-compatible endpoint.

## How It Works

1. **Read existing domains** from the target file (all three sections: available, unavailable, unverified).
2. **Build the prompt** with the user's `--prompt` text, the requested count (`--suggest`), and an exclusion list of existing domains (unless `--fresh` is set).
3. **Call the API** using a tool named `suggest_domains` with `tool_choice` forced, ensuring the model returns structured JSON.
4. **Parse the response** from `choices[0].message.tool_calls[0].function.arguments`.
5. **Normalize and deduplicate** each suggestion via `normalizeDomain()`, then append only new domains to the `unverified` array.
6. **Auto-verify** (optional): if a WHOIS server is configured and `--no-verify` is not set, immediately run WHOIS checks on the new suggestions.

## Prompt Structure

**System prompt:**
```
You generate domain name ideas. All domain names must end with .com. Do not return any domain without .com.
```

**User prompt (with exclusions):**
```
<user prompt> Return <N> unique domain suggestions in the 'unverified' array. Each domain must end with .com. Do not return any domain without .com. Do NOT suggest any of these existing domains: <comma-separated list>
```

## Domain Normalization

Every suggestion passes through `normalizeDomain()` which:

- Lowercases and trims whitespace
- Strips repeated `.com.com` suffixes
- Collapses double dots (`..` → `.`)
- Rejects domains not ending in `.com`
- Rejects domains with more than two dot-separated parts (no subdomains)
- Validates the label with `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$` (no special chars, no leading/trailing hyphens)

## Parallel Requests

`--suggest-parallel N` fires N concurrent API requests simultaneously, each requesting the same count. Results are merged and deduplicated after all complete. See [Parallel Processing](parallel-processing.md).

## Auto-Verification

After suggestions are written, if a WHOIS server is configured and `--no-verify` is not set, the tool automatically calls `RunCLIGroupedInput()` with a 100ms sleep to verify the unverified domains via WHOIS.

## `TALIA_SUGGEST` Env Var Behavior

If `--suggest` is not set explicitly but `TALIA_SUGGEST` is in the environment:
- **Used** if the target file has no pending `unverified` domains.
- **Ignored** if the file already has `unverified` entries (prevents double-suggesting mid-workflow).
- The explicit `--suggest` flag always fires regardless.

## Limitations

- Hardcoded to `.com` domains only (enforced in both the prompt and validation).
- Parallel requests receive the same exclusion list (pre-run snapshot), so they may return overlapping suggestions. File-level dedup resolves this.
- The default model (`gpt-5-mini`) must be overridden via `--model` or `TALIA_MODEL` if it doesn't match your provider's model catalog.

## Related Documentation

- [ADR-002: AI Suggestion Architecture](../decisions/002-ai-suggestion-architecture.md)
- [Parallel Processing](parallel-processing.md)
- [Configuration Reference](../guides/configuration.md)
- [Domain Checking](domain-checking.md)
