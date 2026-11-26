---
name: audit-doc-drift
description: Where markdown or inline docs no longer match the code
argument-hint: "<code path to check>"
allowed-tools:
  - Bash(git log *)
  - Bash(kit adrs *)
---

# /audit-doc-drift

Detect drift between code and markdown or inline documentation.

## Run

With no target in the arguments, ask for one first, since the auditor can't ask. Resolve a linked ticket or doc as `skills/investigate` step 0 describes, then work this file's procedure on the resolved arguments per `skills/kit-rules/reference/subagents.md` section 4, and write the report yourself. Then run the report-format skill's Runtime checks yourself, and reply per `skills/kit-rules/reference/output.md` section 3.

## Prerequisites

$ARGUMENTS should point to the code surface to check. Required.

## Procedure

1. Stack is in the injected `<repo-context>` block.
2. Read the code at $ARGUMENTS.
3. Find relevant docs. Prefer targeted over exhaustive:
   - `README.md` at the project root if $ARGUMENTS is in `src/` or equivalent
   - `docs/` folder, but only files whose name or first heading mentions the target area
   - JSDoc, docstrings, and inline comments in the target files
   - `CHANGELOG.md` only if the task involves a version boundary
4. If `<root>/.claude/rules/conventions/*.md` exists, check every `path:line` citation in it, whatever $ARGUMENTS names. Read the cited line with a few lines of context and confirm it still shows the convention its bullet states. Moved within the file: `warning`, with the new line. Gone, or now showing the opposite: `failure` against the rule file's line, since the rule now misleads.
5. Skip the dirs `skills/kit-rules/reference/tooling.md` section 1 lists, and `LICENSE.md`.
6. Accepted decisions: `kit adrs <target>` lists the ADRs that cite the target, and an accepted RFC or design doc in the project's design folder that names it counts too. Read each one's Decision, and check the code at the target still follows it. `kit adrs` failing means the project keeps no ADRs.
7. Compare.

## What counts as drift

- Inline comments or JSDoc that describe code no longer present or that has changed behavior
- Parameter descriptions that no longer match the signature
- Return type docs that conflict with the code's declared types
- External markdown with usage examples that would fail if run
- Configuration instructions that reference renamed or removed options
- Documented features that have been removed or renamed
- Feature documented but no longer exists
- Code that contradicts an accepted ADR, RFC, or design doc with nothing superseding it: `failure` against the code's `file:line`. The fix is the code, or a new ADR that supersedes the old one (`/write-adr`).
- Requirements documented in README, CLAUDE.md, or design docs that no longer match the implementation. Examples: a "supports X" claim with no code implementing X; a "must validate Y" rule with no validation in the relevant handler; a documented invariant that the code violates.

## What does not count as drift

- Missing documentation for new features (that is a docs gap, not drift)
- Stylistic differences (tone, format)
- Out of date links to external sites, unless the target area depends on them

## Proof

Each finding quotes both sides with `file:line`: the doc's claim and the code that contradicts it. For an example that would fail, name the line of code that makes it fail; never run it.

## Output per finding

- What it is, with the documentation's `file:line`
- Evidence: what the documentation says (a short quote) and what the code does, with its `file:line`
- Severity: `failure` when it actively misleads, `warning` when it's partly outdated, `info` for a cosmetic gap
- Fix: the correction, noted and never applied

## Output file

Load the report-format skill and use its format. Write to the path `kit scratch-dir doc-drift <target-slug>` prints.

## Scope rules

- Do not edit documentation during the audit.
- Do not flag "missing documentation" as drift. Only flag documented claims that no longer hold.
- If nothing drifted, the report says "No drift detected" and lists what was checked so the user knows the scope covered.
