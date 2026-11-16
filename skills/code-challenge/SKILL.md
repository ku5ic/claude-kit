---
name: code-challenge
description: Challenge a planned or finished change's decisions the way a senior reviewer would
argument-hint: "[plan or report path | ticket URL | PR number | branch | nothing]"
disable-model-invocation: true
allowed-tools:
  - Bash(gh pr diff *)
  - Bash(gh pr view *)
  - Bash(glab mr diff *)
  - Bash(glab mr view *)
  - Bash(git log *)
  - Bash(kit code-graph *)
  - Bash(kit git-base *)
  - Bash(kit scratch-dir *)
---

# /code-challenge

Challenge a proposed or finished change the way a senior reviewer would, decision first and code second. Most useful before the change is written.

## Inputs

Resolve the artifact, first match wins:

1. A file path: a plan or a report.
2. A ticket URL: resolved as `investigate` step 0 describes.
3. A PR number or branch: its title, body, changed files, and diff, through the code host per `skills/kit-rules/reference/tooling.md` section 4.
4. Nothing: the current plan file in plan mode, else the newest report in `kit scratch-dir`, else `kit git-base --diff`.

If the artifact links a ticket, fetch that too; the "why" lives there. Read the project's CLAUDE.md and load the pattern skills it suggests.

ADRs, when they exist: `adr list` from the repo root if it has an `.adr-dir` (without `adr`, the files in the folder it names). `rg` them for the units and terms the artifact touches; a hit feeds checks 2 and 4.

Mode follows the input:

- **Pre-change** (plan, report, ticket): check against the files and symbols the artifact names, read from the repo.
- **Post-change** (diff, PR): check against the diff.

## Decision pass (blocking)

Run in this order. Answer each from the repo first; only what the repo can't answer becomes a finding.

1. **Purpose.** Is the why stated in one sentence? Is the change needed at all?
2. **What changes vs before.** Compare current and proposed behavior: what's shown, hidden, or allowed, and under which conditions. `git log -S` or blame on the old behavior to learn why it existed before replacing it.
3. **Blast radius and root cause.** Count consumers of every shared unit touched, as `skills/code-impact/SKILL.md` Consumers does. Is the fix at the root, or in a shared layer whose other callers aren't broken?
4. **Domain meaning.** Which fields, states, or terms does the change assume a meaning for? Are they used that way elsewhere?
5. **Existing solution.** Does the repo already have something that does this?
6. **Usage.** Does every new export, option, or field have a consumer in this change?
7. **Breaking it.** List the cases from `skills/kit-rules/reference/verify.md` section 2's axes that apply. Don't run them. Pre-change: does the plan's verification cover them?
8. **Scope and dependencies.** Does part of this belong in a ticket, an upstream change, or a redesign? Is an upstream contract or design the change relies on actually in place?

## Convention pass (non-blocking)

Only from the project's own rules, CLAUDE.md, and loaded pattern skills, never generic taste. Pre-change, check only the paths, names, and structure the artifact proposes.

## Filter

Drop every finding the repo or the artifact already answers. Keep 3 to 7 blocking findings. Fewer is fine when the change is sound.

## Report

Load the report-format skill and use its format, written to the path `kit scratch-dir challenge <artifact-slug>` prints, with these narrowings:

- The Scope line names the artifact and the mode, and says "no ADRs found" when the repo has none.
- The TL;DR's first bullet is the verdict line, in place of the health word:
  - Post-change: `LGTM, a few non-blocking nits` / `LGTM, but <blocker>` / `let's discuss first: <decision>`
  - Pre-change: `go` / `go after: <question>` / `let's discuss first: <decision>`
- Findings from the decision pass come first, the check named after the severity (`warning: Blast radius, ...`). Severity is `failure` when the evidence shows a defect, `warning` for an open question that blocks. Convention-pass findings are `info`.
- Each finding's location is a `file:line` post-change, or the artifact's section or step pre-change.
- Each finding is shaped and worded per `skills/kit-rules/reference/output.md` section 4.

## Reply

The report path and the verdict line.

## Stop

Read-only. Never edit, never comment on a PR or ticket, never approve.

Arguments: `$ARGUMENTS`
