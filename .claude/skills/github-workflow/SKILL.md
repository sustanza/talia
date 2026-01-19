---
name: github-workflow
description: Handles GitHub commit, issue, and PR workflows. Use for creating commits that close issues, opening issues with ACs, and creating PRs. Integrates with gh CLI.
allowed-tools: Bash, Read, Glob, Grep
---

# GitHub Workflow

## Commits

### Format
Use conventional commits. No co-author lines.

```
<type>: <subject>

<body>

<footer>
```

Types: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`

### Before Committing
1. Verify ALL acceptance criteria in the issue are met
2. Build and test pass
3. README updated if features changed

### HEREDOC Format
Always use HEREDOC for multi-line commit messages:

```bash
git commit -m "$(cat <<'EOF'
feat: implement feature X

- Detail one
- Detail two

Closes #19
EOF
)"
```

## Closing Issues

Use GitHub keywords in commit messages to auto-close issues on push:

- Single: `Closes #19`
- Multiple: `Closes #19, closes #20, closes #21`
- Keywords: `closes`, `fixes`, `resolves` (case-insensitive)

Keywords only work when pushed to the default branch.

## Creating Issues

Use gh CLI with full details:

```bash
gh issue create \
  --title "Build FeatureView" \
  --label "feature/area" \
  --milestone "M2: CRUD Complete" \
  --body "$(cat <<'EOF'
## Technical Notes
- Implementation details
- Architecture decisions

## Acceptance Criteria
- [ ] AC one
- [ ] AC two
EOF
)"
```

## Pull Requests

### Create PR
```bash
gh pr create \
  --title "feat: add feature X" \
  --body "$(cat <<'EOF'
## Summary
- What changed and why

## Test Plan
- [ ] How to verify

Closes #19
EOF
)"
```

### Review PR
```bash
gh pr view <number>
gh pr diff <number>
gh pr review <number> --approve
gh pr merge <number>
```

## Best Practices

- One logical change per commit
- Link related issues with `#` references
- Verify ACs before closing issues
- Push promptly so issues close automatically
