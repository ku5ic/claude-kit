---
name: audit-tokens
description: Design token audit, hard-coded values a token covers, unused, undefined, and duplicate tokens, and token color contrast
argument-hint: "<path, or nothing for the project root>"
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
  - Bash(kit contrast *)
---

# /audit-tokens

Audit how a codebase uses its design tokens. Read-only.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                    | Read                                                   |
| ------------------------- | ------------------------------------------------------ |
| Finding the token sources | [reference/token-files.md](reference/token-files.md)   |
| Running the checks        | [reference/audit-checks.md](reference/audit-checks.md) |

## Run

Resolve the target, the arguments or the project root, then work this file's procedure per `skills/kit-rules/reference/subagents.md` section 4, and write the report yourself. Then run the report-format skill's Runtime checks yourself, and reply per `skills/kit-rules/reference/output.md` section 3.

## Procedure

1. Stack is in the `<repo-context>` block. Load the pattern skill for each UI stack in it (tailwind-patterns for `@theme`).
2. Find the token sources per [reference/token-files.md](reference/token-files.md), and list each token with its name, value, file, and mode. Generated outputs aren't sources.
3. Run the five checks in [reference/audit-checks.md](reference/audit-checks.md), hard-coded, unused, undefined, duplicate, and contrast.
4. Contrast comes from `kit contrast <fg> <bg>` only, never hand arithmetic. The thresholds are the wcag-audit skill's.

## Output file

Load the report-format skill and use its format. Write to the path `kit scratch-dir tokens <target-slug>` prints. Each finding carries `file:line`, the value or token, and the token that should have been used.

## Rules

- No token file, config, or source is edited.
- A token or value the checks can't resolve is `unverified`, never a pass.

Arguments: `$ARGUMENTS`
