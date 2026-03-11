# ADR-002: AI Suggestion Architecture

**Date:** 2025-05-29
**Status:** Accepted

## Context

Talia needs to generate creative domain name suggestions. Rather than using hardcoded word lists or algorithmic generation, the tool leverages large language models via the OpenAI-compatible chat completions API.

## Decision

Use the OpenAI **tool-calling (function calling)** API format to enforce structured output.

Key design choices:

1. **Tool calling over `response_format`** — A single tool named `suggest_domains` is defined with a JSON schema. The model is forced to call it via `tool_choice`, guaranteeing structured output regardless of model provider.

2. **OpenAI-compatible API** — The `--api-base` flag (or `OPENAI_API_BASE` env var) allows pointing to any compatible endpoint (OpenAI, Gemini, local models), making the tool provider-agnostic.

3. **`.com` only** — The system prompt and `normalizeDomain()` validation both enforce `.com` TLD exclusively. This is a deliberate constraint to keep the tool focused.

4. **Exclusion list** — Existing domains from the file are passed to the AI as "do not suggest these" (unless `--fresh` is set), reducing duplicates at the prompt level.

5. **Post-generation deduplication** — `writeSuggestionsFile()` also deduplicates against the full file contents, catching anything the model still repeats.

## Alternatives Considered

1. **Structured output via `response_format`** — Less portable across providers; tool calling is more widely supported.
2. **Embedding-based similarity search** — Over-engineered for name generation; doesn't leverage model creativity.
3. **Multi-TLD support** — Increases complexity significantly (different WHOIS servers per TLD, validation rules). Deferred.

## Consequences

- **Pro:** Works with OpenAI, Gemini, and any OpenAI-compatible API out of the box.
- **Pro:** Structured output is guaranteed by the tool-calling contract.
- **Pro:** Deduplication happens at two levels (prompt exclusion + file-level seen map).
- **Con:** Parallel requests all receive the same exclusion list (pre-run snapshot), so they may generate overlapping suggestions. File-level dedup catches this.
- **Con:** Hardcoded to `.com` — extending to other TLDs requires changes in both the prompt and validation logic.

## Related Documentation

- [AI Suggestions](../features/ai-suggestions.md)
- [Parallel Processing](../features/parallel-processing.md)
- [Configuration Reference](../guides/configuration.md)
