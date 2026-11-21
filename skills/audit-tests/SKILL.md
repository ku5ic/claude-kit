---
name: audit-tests
description: Tests that add code and catch no regression, each with the cut, merge, or rewrite that fixes it
argument-hint: "<optional: a path, or --all for the whole suite>"
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
  - Bash(kit git-base *)
---

# /audit-tests

Find the tests that cost code and catch nothing: ones that can't fail, ones bound to the implementation, duplicates, and trivial ones. Missing coverage is `/audit-debt`'s Testing debt.

## Run

Resolve the target, then dispatch the auditor agent (subagent_type: kit:auditor, foreground) with this file's procedure and the resolved target. It writes the report. Then run the report-format skill's Runtime checks yourself, and reply per `skills/kit-rules/reference/output.md` section 3.

The target, first match wins:

- `--all`: every test file in the repo.
- A path: the test files under it.
- Nothing: the test files `kit git-base --diff --name-only` lists. None, say so and stop.

## Procedure

1. Load `test-patterns`, and the pattern skill for the stack the `<repo-context>` block names. Read `test-patterns`' `reference/principles.md`, `reference/anti-patterns.md`, and `reference/coverage-and-skip.md`. They hold every criterion below; this file only says how to apply them.
2. Read each test in the target, and the code it calls.
3. Flag a test of one of these kinds:
   - **Can't fail**: no assertion, an assertion always true or always false, or an unawaited async assertion (`anti-patterns.md`).
   - **Tests the implementation**: asserts on internals, call counts of private functions, or a mock of every import (`principles.md`, `anti-patterns.md`).
   - **Duplicate**: a copy of another test with only its inputs changed, or the same condition asserted twice (`anti-patterns.md`).
   - **Trivial**: covers a pass-through wrapper, a getter or setter, or a framework default (`coverage-and-skip.md`).
4. A finding comes from reading the test. Quote the lines that make it one of the four kinds.
5. For a test added or changed since `kit git-base` whose subject also changed, name one runtime check under "Needs a runtime check": the test run alone against the base's code, in a worktree at `"$(kit scratch-dir)/tests-base"` with the branch's test file copied in, using the project's own test command. It touches only that worktree, removed with `git worktree remove --force` after. A test that passes there too caught none of the branch's change.

## Not a finding

- A test the project's coverage rules require for a business rule, auth, payment, or security boundary, however plain it reads (`coverage-and-skip.md`).
- A snapshot of stable serialized output (`anti-patterns.md`).

## Severity anchors

`anti-patterns.md` gives each kind its level. A trivial test is `info`.

## Output per finding

- What it is, with `file:line`, and its kind
- Evidence: the quoted lines, or the runtime check's command and output once it ran
- Why it matters: the code it costs, and the regression it would miss
- Fix, one of:
  - delete it, with the lines saved
  - merge it into a table-driven or parametrized test, naming the rows
  - rewrite it to assert behavior, naming the behavior
  - the regression the suite still misses once it's gone, as the test to write

## Output file

Load the report-format skill and use its format. Write to the path `kit scratch-dir tests <target-slug>` prints.

## Rules

- Change no test. The report is the only output.
- A test that passes on the base's code is confirmed as catching nothing only after its runtime check ran.
