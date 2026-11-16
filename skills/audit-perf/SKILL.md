---
name: audit-perf
description: Performance audit of a file, directory, or area, for costs a static read can find
argument-hint: "<file, directory, area name, or a link to an external tracker/doc>"
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
---

# /audit-perf

Performance audit focused on statically detectable issues.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                                           | Read                                             |
| ------------------------------------------------ | ------------------------------------------------ |
| Writing a finding's evidence, or what to measure | [reference/measuring.md](reference/measuring.md) |

## Run

With no target in the arguments, ask for one first, since the auditor can't ask. Resolve a linked ticket or doc as `skills/investigate` step 0 describes, then dispatch the auditor agent (subagent_type: kit:auditor, foreground) with this file's procedure and the resolved arguments. It writes the report; reply per `skills/kit-rules/reference/output.md` section 3.

## Procedure

1. Stack is in the `<repo-context>` block from the `SubagentStart` hook.
2. Load the pattern skill for each stack in it, and apply its performance and anti-pattern references. A stack with no pattern skill: work from the project's own configs and lint rules, and say so in the report.
3. Review the target across these categories:
   - Ground each candidate in this project's precedent, per `skills/kit-rules/reference/change.md` section 2: a pattern that repeats elsewhere unflagged by CLAUDE.md or tests is a deliberate tradeoff, not a finding.
   - Exception: flag it anyway if it is measurably worse at this location than elsewhere.
4. Skip categories with no findings.

### UI and client

- Work repeated on every render: a large subtree re-rendering for a change only a leaf uses, a shared value whose churn re-renders every consumer
- Missing memoization where it actually matters (measurably expensive computation, or a stable reference a memoized consumer needs)
- Item identity on lists that reorder
- Waterfalls: effects or fetches that wait on each other when they could be one, or run in parallel
- Client payload: a large library, or server-only code through a shared utility, reaching the client bundle; imports nothing uses
- Layout shift from media without dimensions; above-the-fold content that renders only on the client

### Data and request path

- N+1 queries: a query per row of a list
- Unbounded queries: no pagination or limit on a table that grows
- Synchronous I/O in an async context
- Missing indexes on filtered or ordered columns (static inference from query shape)
- Serialization cost: deep nesting, recursive expansion
- Cache misuse: caching with key collisions, caching before query filtering

### General

- Unbounded loops
- Accidental O(n^2) via nested iteration over the same collection
- Reading a whole file when streaming is possible
- Blocking operations in request path

## Proof

Each finding quotes the code and names what makes it cost: the loop bound around a query, the list size a render walks, the package an import pulls into the client bundle, with its size on disk.

## Not a finding

- A cost that only a developer pays once, such as a slow codegen or a cold dev server start, unless the target is that tool.
- A pattern without a size behind it: a nested loop over two fixed, small lists.

## Severity

- `failure`: unbounded work on a request or render path, such as a query per row of a user-sized list, or an unpaginated query.
- `warning`: bounded but wasteful on a hot path, such as a large subtree re-rendering on every keystroke.
- `info`: worth measuring, with no evidence yet that it costs.

## Verification tools

- Command-level perf checks (build scripts, codegen, test runner startup, CLI tools): measure with `hyperfine`, not eyeballed ([reference/measuring.md](reference/measuring.md)).
- Render-level perf (re-renders, bundle cost): measure with the UI framework's profiler or the browser's performance panel at runtime, not from static analysis.

## Output per finding

- What it is, with `file:line`: the specific pattern
- Evidence: what was checked and what it showed (a render count, a `hyperfine` result, the quoted line). When the cost isn't statically obvious, how to measure it ("profile it, expect a re-render on every parent state change")
- Why it matters: estimated impact (render count, query count, payload size)
- Severity: per the report-format skill's rubric
- Fix: the concrete code change or pattern switch

## Output file

Load the report-format skill and use its format. Write to the path `kit scratch-dir perf <target-slug>` prints.

## Rules

- Do not claim measured improvement. This command does not run anything.
- Flag what to measure, not what to assume.
- Ignore micro-optimizations that change code without measurable benefit.
- A pattern that repeats across the codebase as an established choice is not N separate findings; note it once against the shared source and list the consuming locations.

Arguments: `$ARGUMENTS`
