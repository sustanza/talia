# Talia Documentation

Documentation index for the Talia CLI — a domain availability checker and AI-powered domain suggestion tool.

## Structure

```text
docs/
  README.md                          # this file — master index
  decisions/                         # architecture and technical rationale
    001-whois-availability-detection.md
    002-ai-suggestion-architecture.md
    003-parallel-processing-design.md
    004-output-format-design.md
  features/                          # product behavior and rules
    domain-checking.md
    ai-suggestions.md
    file-cleaning.md
    merge-and-export.md
    parallel-processing.md
  guides/                            # development and operations
    development.md
    configuration.md
    testing.md
  plans/                             # open risks and known issues
    known-issues.md
  templates/                         # authoring skeletons
    decision-record.md
    feature-spec.md
    guide.md
    plan.md
```

## Doc Type Taxonomy

| Question | Folder |
|---|---|
| Why did we choose this architecture? | [decisions/](decisions/) |
| How should the feature behave? | [features/](features/) |
| How do I develop, configure, or test? | [guides/](guides/) |
| What is pending, risky, or open? | [plans/](plans/) |
| How do I author a new doc? | [templates/](templates/) |

## Decision Records

- [001 — WHOIS Availability Detection](decisions/001-whois-availability-detection.md)
- [002 — AI Suggestion Architecture](decisions/002-ai-suggestion-architecture.md)
- [003 — Parallel Processing Design](decisions/003-parallel-processing-design.md)
- [004 — Output Format Design](decisions/004-output-format-design.md)

## Feature Specs

- [Domain Checking](features/domain-checking.md) — WHOIS-based availability verification
- [AI Suggestions](features/ai-suggestions.md) — AI-powered domain name generation
- [File Cleaning](features/file-cleaning.md) — domain normalization and deduplication
- [Merge and Export](features/merge-and-export.md) — file merging and plain text export
- [Parallel Processing](features/parallel-processing.md) — concurrent WHOIS and suggestion requests

## Guides

- [Development Guide](guides/development.md) — building, running, and contributing
- [Configuration Reference](guides/configuration.md) — all flags, env vars, and `.env` support
- [Testing Guide](guides/testing.md) — test architecture, mocking, and CI

## Plans

- [Known Issues](plans/known-issues.md) — open risks and quirks

## Authoring Rules

1. One primary topic per file.
2. Stable heading hierarchy (`#`, then `##` for key sections).
3. Use relative links for local navigation.
4. Add a `Related Documentation` section near the end.
5. Use explicit dates (`YYYY-MM-DD`) when time matters.
6. Prefer bullet constraints over long prose when defining rules.
7. Keep examples concrete (paths, commands, model names, flows).

## Maintenance

- Update affected docs in the same PR as code changes.
- If architecture changed, update `decisions/` first, then link from feature docs.
- If behavior changed, update `features/` and related guides.
- Run markdown link validation after structural changes.
