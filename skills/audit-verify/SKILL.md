---
name: audit-verify
description: Re-check a prior audit report's findings against the repo as it is now
argument-hint: "<path to an existing audit report>"
allowed-tools:
  - Bash(git log *)
---

# /audit-verify

Re-check findings from a prior audit report against the current repo, classifying each as resolved, unresolved, moved, regressed, or unverifiable.

## Procedure

1. Resolve the report path from $ARGUMENTS. If empty or the file does not exist, list the most recent reports (`ls -t "$(kit scratch-dir)"/*.md | head -10`) and ask which one via AskUserQuestion rather than guessing.
2. Read the report in full. Identify its kind from the filename prefix (`a11y-`, `arch-`, `perf-`, `debt-`, `doc-drift-`, `security-`, `ci-`, `infra-`, `slop-`, `tests-`, `tokens-`, `flags-`, `skills-`) or, absent a conventional filename, its title/Scope line. If the kind matches none of these, stop and ask rather than guessing.
3. Extract every finding's severity (its heading), location and problem (its first sentence), Evidence, Why it matters, Fix, and Refs from `## Findings`, per the report-format skill's finding shape. An older report carries severity, location, and problem as `Severity`, `Location`, and `What` fields.
4. Dispatch the auditor agent (subagent_type: kit:auditor), one per group of findings per `skills/kit-rules/reference/subagents.md` section 4, each with the report kind, the skills the kind's reference loads, and only its group of the extracted findings, not the full original report, plus an explicit instruction to re-check each cited location as it exists in the repo right now. This is a re-check, so it must not scan for new findings outside the given list.
5. For each finding, its agent classifies:
   - `resolved`: the described problem is no longer present at the cited location.
   - `unresolved`: the described problem is still present as described.
   - `moved`: the problem still exists but the cited location no longer matches; the agent must find and cite the new location.
   - `regressed`: evidence (e.g. git history at that location) shows it was fixed and then reintroduced.
   - `unverifiable`: the original finding had no `file:line` (a free-text region, per the report-format skill's own allowance), always reported, never dropped.
6. Relay the agents' classifications into the output report below, one per original finding, with no merge, no severity change, and no new tracing. Do not re-run any part of the original audit yourself.

## Output

Write to the path `kit scratch-dir verify <original>` prints, where `<original>` is the original report's file name without `.md`, unique in its scratch dir. Never modify the original report: this command is append-only across files, so a bad verify run cannot corrupt an audit.

```markdown
# Verify: <original report filename>

Generated: <ISO timestamp>
Original report: <absolute path>

## TL;DR

- <counts per classification, e.g. "3 resolved, 1 unresolved, 1 moved, 0 regressed, 1 unverifiable">
- <the first unresolved or regressed failure, with its location>

---

## Classifications

### <classification>: <finding title>

<One sentence: the original severity and location, and the current location if moved.>

- **Evidence:** <what was checked and what it showed>

### <Next finding>

...
```

## Rules

- Never edit or delete the original report.
- A finding with no `file:line` is `unverifiable`, not silently dropped and not guessed at.
- Do not expand scope into a fresh audit of the surface; unresolved findings outside the original list are out of scope for this command.
