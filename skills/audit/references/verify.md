# /audit verify

Re-check findings from a prior audit report against the current repo, classifying each as resolved, unresolved, moved, regressed, or unverifiable. Arguments: `<path to an existing audit report>`.

## Procedure

1. Resolve the report path from $ARGUMENTS. If empty or the file does not exist, list the most recent reports (`ls -t "$(scratch-dir.sh)"/\*.md | head -10`) and ask which one via AskUserQuestion rather than guessing.
2. Read the report in full. Identify its kind from the filename prefix (`security-`, `a11y-`, `perf-`, `debt-`, `doc-drift-`) or, absent a conventional filename, its title/Scope line. If the kind matches none of these, stop and ask rather than guessing.
3. Extract every finding's Severity, Location, What, Why it matters, Fix, and Refs fields from `## Findings` (`rules/markdown-report.md`'s required per-finding shape).
4. Dispatch the auditor agent (subagent_type: auditor, foreground) with the report kind, the checklist skill it implies (wcag-audit for a11y, security-patterns for security, the stack patterns skill otherwise), and only the extracted finding list, not the full original report, plus an explicit instruction: re-check each cited location as it exists in the repo right now. This is a re-check, not a re-audit -- it must not scan for new findings outside the given list.
5. For each finding, the agent classifies:
   - `resolved`: the described problem is no longer present at the cited location.
   - `unresolved`: the described problem is still present as described.
   - `moved`: the problem still exists but the cited location no longer matches -- the agent must find and cite the new location.
   - `regressed`: evidence (e.g. git history at that location) shows it was fixed and then reintroduced.
   - `unverifiable`: the original finding had no `file:line` (a free-text region, per `rules/markdown-report.md`'s own allowance) -- always reported, never dropped.
6. Relay the agent's classifications into the output report below. Do not re-run any part of the original audit yourself.

## Output

Write to `$(scratch-dir.sh)/verify-<original-slug>-<YYYYMMDD-HHMM>.md`, where `<original-slug>` is the original report's own target slug, or its full kind-and-timestamp stem if it has none. Never modify the original report -- this command is append-only across files, so a bad verify run cannot corrupt an audit.

```
# Verify: <original report filename>

Generated: <ISO timestamp>
Original report: <absolute path>

## Classifications

### <Finding title>

- Original severity: <failure | warning | info>
- Original location: <file>:<line> or <region>
- Classification: <resolved | unresolved | moved | regressed | unverifiable>
- Current location: <file:line, if moved; omit otherwise>
- Evidence: <what was checked and what it showed>

### <Next finding>
...

## Summary

<counts per classification, e.g. "3 resolved, 1 unresolved, 1 moved, 0 regressed, 1 unverifiable">
```

Print the path.

## Rules

- Never edit or delete the original report.
- A finding with no `file:line` is `unverifiable`, not silently dropped and not guessed at.
- Do not expand scope into a fresh audit of the surface; unresolved findings outside the original list are out of scope for this command.
