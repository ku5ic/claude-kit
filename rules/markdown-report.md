---
paths:
  - "**/scratch/**/*.md"
---

# Markdown report format

Consistent format for findings reports: audits, reviews, dependency runs, and any other structured list of problems found. A skill that defines its own report template (`explore-patterns`) owns its shape; only the file-naming, path-printing, and ASCII rules below apply to those.

## Required sections

```
# <Report type>: <target>

Generated: <ISO timestamp>
Scope: <file, component, or module>
Stack: <line from $HOME/.claude/bin/detect-stack.sh, if applicable>

## Summary

<One paragraph. What was checked, what was found, overall health in one word.>

## Findings

### <Finding title>

- Severity: <failure | warning | info>
- Location: <file>:<line> or <region>
- What: <problem in one or two sentences>
- Why it matters: <one sentence>
- Fix: <code snippet or concrete instruction>
- Refs: <WCAG criterion, CVE, doc link, etc., if relevant>

### <Next finding>
...

## Cannot be verified statically

<Items that need runtime checks, user testing, or external tools. Omit section if empty.>

## Out of scope

<Things noticed but not part of the task. Omit if empty.>
```

## Rules

- Severity is one of `failure`, `warning`, `info`. Nothing else. Do not invent new levels.
- Sort findings by severity, failures first.
- If a section would be empty, omit it. Do not leave placeholder text.
- Code snippets use fenced blocks with language tag.
- No ASCII decoration, no banner comments, no emoji.
- Use straight quotes, plain ASCII punctuation.
- File naming and location: per `rules/tooling.md` - resolve the directory via `scratch-dir.sh`, then name it `<kind>-<target-slug>-<YYYYMMDD-HHMM>.md`.
- Always print the absolute path at the end of execution so the user can open it.
- Does not govern `/write` output (commit messages, PR descriptions, release notes, stakeholder summaries) - those have their own formats per `rules/output.md`.

## Summary line rubric

The "overall health in one word" at the end of the Summary helps quick scanning:

- `clean` (no findings)
- `minor` (only info)
- `moderate` (warnings, no failures)
- `serious` (failures present)
- `broken` (multiple critical failures, work should pause)

## Anti-patterns

- `failure`: writing the report body to terminal output instead of a file -- the artifact becomes ephemeral and unreferenceable.
- `failure`: omitting the `## Summary` section or the `## Findings` section when findings exist, in a report this format governs.
- `warning`: inventing severity levels outside `failure`, `warning`, `info` -- e.g. `critical`, `high`, `medium`, `low`, `error`. The rubric has three levels; anything else breaks downstream tooling that parses reports.
- `warning`: leaving placeholder text in empty sections (e.g. `<none>`, `N/A`) rather than omitting the section.
- `warning`: hardcoding a literal `~/.claude/scratch/` or `scratch/` path instead of resolving it via `scratch-dir.sh`. The resolved directory is project-scoped inside a recognized project and home-fallback otherwise - a literal is wrong in whichever case it doesn't match.
- `warning`: not printing the absolute file path after writing -- the user cannot open the file without it.
- `info`: not sorting findings by severity (failures first, then warnings, then info).
