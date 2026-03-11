# Universal Documentation System Playbook

A reusable documentation architecture you can apply to most software projects.

## Purpose

This playbook defines a documentation system that is:
- easy to navigate
- hard to duplicate
- clear about ownership
- scalable as product and team complexity grows

It separates documentation by intent ("why", "how", "workflow", "history", "external copy") so readers can find the right source of truth fast.

## Core Design Principles

1. Single source of truth per concern.
2. Separate rationale from behavior from implementation workflow.
3. Keep indexes opinionated and concise.
4. Prefer cross-links over repeated content.
5. Evolve with templates and explicit maintenance rules.

## Recommended Directory Structure

```text
docs/
  README.md                 # master index and taxonomy
  decisions/                # architecture and technical rationale
  features/                 # product behavior and platform rules
  guides/                   # standards and implementation playbooks
  plans/                    # audits, migration plans, open risks
  marketing/                # external copy and review responses
  prompts/                  # prompt assets/specs (if AI is used)
  templates/                # authoring templates for all doc types
```

## Folder Contracts

### `docs/README.md`

Must include:
- structure map (tree)
- doc-type taxonomy
- links to each section index or key docs
- authoring and maintenance rules

Purpose:
- onboarding entrypoint for all contributors

### `docs/decisions/`

Use for:
- architectural choices
- alternatives considered
- tradeoffs and consequences

Do not use for:
- step-by-step implementation tasks
- user-facing behavior details

### `docs/features/`

Use for:
- expected product behavior
- platform differences
- edge cases and state handling

Do not use for:
- architecture debates (link to decisions instead)
- contributor workflows (link to guides instead)

### `docs/guides/`

Use for:
- durable standards (design, content, quality bars)
- implementation playbooks
- contributor runbooks

Typical examples:
- design philosophy
- widget/module development guide
- integration or release checklist guide

### `docs/plans/`

Use for:
- audit snapshots
- migration plans
- unresolved items and risk tracking

Rule:
- explicitly date and status plan documents

### `docs/marketing/`

Use for:
- app store copy
- launch messaging
- reviewer responses

Rule:
- isolate from engineering specs to avoid cross-purpose edits

### `docs/prompts/` (optional)

Use for:
- prompt contracts
- reusable prompt assets

Rule:
- keep outputs deterministic in shape (sections, fields, constraints)

### `docs/templates/`

Use for:
- canonical authoring skeletons

Rule:
- update templates whenever recurring section standards change

## Doc Type Decision Matrix

Use this to decide where new content belongs:

- "Why did we choose this architecture?" -> `decisions/`
- "How should the feature behave?" -> `features/`
- "How do I implement or operate this?" -> `guides/`
- "What is pending/risky/open?" -> `plans/`
- "What do users/reviewers read externally?" -> `marketing/`
- "How should prompts be written?" -> `prompts/`

## Standard Template Set

At minimum, maintain templates for:
- docs index/readme
- decision record
- feature spec
- plan/audit
- marketing doc
- philosophy/standards doc
- implementation guide
- prompt doc (if applicable)

## Authoring Rules (Universal)

1. One primary topic per file.
2. Stable heading hierarchy (`#`, then `##` for key sections).
3. Use relative links for local navigation.
4. Add `Related Documentation` section near the end.
5. Use explicit dates (`YYYY-MM-DD`) when time matters.
6. Prefer bullet constraints over long prose when defining rules.
7. Keep examples concrete (paths, commands, model names, flows).

## Link and Consistency Rules

1. Every major doc is reachable from `docs/README.md` within 2 clicks.
2. Every feature doc links to at least one related decision or guide.
3. Every decision links to affected feature docs.
4. Every guide links to relevant feature/decision docs.
5. Run markdown link validation after structural changes.

## Governance Model

Define owners explicitly:
- `decisions/`: architecture owner or tech lead
- `features/`: product + engineering owner for that domain
- `guides/`: domain maintainer
- `plans/`: project lead or incident owner
- `marketing/`: product/marketing owner
- `templates/`: docs maintainer

Add a simple review cadence:
- monthly docs health check
- pre-release docs audit
- post-incident docs update checklist

## Lifecycle Rules

When code changes:
1. Update affected feature/guide/decision docs in the same PR.
2. If architecture changed, update `decisions/` first, then link from feature docs.
3. If behavior changed, update `features/` and related guides.
4. If language or positioning changed, update philosophy/marketing docs.

When structure changes:
1. update `docs/README.md`
2. update section indexes (for example `docs/guides/README.md`)
3. update templates
4. run link validation

## Migration Playbook (Adopting This in Another Project)

1. Create the baseline folder tree under `docs/`.
2. Add `docs/README.md` with taxonomy and section links.
3. Install template files in `docs/templates/`.
4. Migrate existing docs into `decisions`, `features`, `guides`, `plans`, `marketing`.
5. Replace duplicate content with cross-links.
6. Add section indexes for high-volume folders (for example `guides/README.md`).
7. Add CI or local checks for markdown link validity.
8. Add contribution rule: docs updated in same change as code.

## Anti-Patterns to Avoid

- giant mixed docs containing rationale + behavior + implementation + status
- orphan docs that are not linked from the main index
- duplicate guidance across multiple folders
- template drift where real docs no longer match skeletons
- architecture facts living only in PR descriptions

## Acceptance Checklist

- [ ] `docs/README.md` exists and reflects actual structure
- [ ] each folder has a clear contract
- [ ] templates exist for each active doc type
- [ ] section indexes exist where needed (`guides/README.md`, etc.)
- [ ] no broken relative markdown links
- [ ] at least one owner is accountable for docs quality

## Adaptation Notes

This structure is intentionally domain-agnostic. You can rename folders if needed, but keep the functional separation:
- rationale
- behavior
- implementation workflow
- planning/risk
- external messaging
- templates

If you preserve that separation, the system scales across mobile, backend, infra, and full-stack projects.

## Related Documentation

- [Documentation Index](../README.md)
