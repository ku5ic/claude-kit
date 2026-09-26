# /audit doc-drift

Detect drift between code and markdown or inline documentation. Arguments: `<code path to check>`.

## Prerequisites

$ARGUMENTS should point to the code surface to check. Required.

## Procedure

1. Stack is in the injected `<repo-context>` block. Get the scratch directory: `!`scratch-dir.sh``.
2. Read the code at $ARGUMENTS.
3. Find relevant docs. Prefer targeted over exhaustive:
   - `README.md` at the project root if $ARGUMENTS is in `src/` or equivalent
   - `docs/` folder, but only files whose name or first heading mentions the target area
   - JSDoc, docstrings, and inline comments in the target files
   - `CHANGELOG.md` only if the task involves a version boundary
4. If `<root>/.claude/rules/conventions/*.md` exists, check every `path:line` citation in it, whatever $ARGUMENTS names. Read the cited line with a few lines of context and confirm it still shows the convention its bullet states. Moved within the file: `warning`, with the new line. Gone, or now showing the opposite: `failure` against the rule file's line, since the rule now misleads.
5. Skip: `node_modules/**`, `.next/**`, `coverage/**`, `out/**`, `.turbo/**`, `.cache/**`, `vendor/**`, `target/**`, `dist/**`, `build/**`, `storybook-static/**`, `.pnpm-store/**`, `LICENSE.md`.
6. Compare.

## What counts as drift

- Inline comments or JSDoc that describe code no longer present or that has changed behavior
- Parameter descriptions that no longer match the signature
- Return type docs that conflict with actual TypeScript or Python types
- External markdown with usage examples that would fail if run
- Configuration instructions that reference renamed or removed options
- Documented features that have been removed or renamed
- Feature documented but no longer exists
- Requirements documented in README, CLAUDE.md, or design docs that no longer match the implementation. Examples: a "supports X" claim with no code implementing X; a "must validate Y" rule with no validation in the relevant handler; a documented invariant that the code violates.

## What does not count as drift

- Missing documentation for new features (that is a docs gap, not drift)
- Stylistic differences (tone, format)
- Out of date links to external sites, unless the target area depends on them

## Output per finding

- Location: file and line of the documentation
- What the documentation says (short quote or paraphrase)
- What the code actually does (short description)
- Severity: failure (actively misleads), warning (partially outdated), info (cosmetic gap)
- Suggested correction (not applied; just noted)

## Output file

Load the report-format skill and use its format. Write to the path `scratch-dir.sh doc-drift <target-slug>` prints. Print the path.

## Scope rules

- Do not edit documentation during the audit.
- Do not flag "missing documentation" as drift. Only flag documented claims that no longer hold.
- If nothing drifted, the report says "No drift detected" and lists what was checked so the user knows the scope covered.
