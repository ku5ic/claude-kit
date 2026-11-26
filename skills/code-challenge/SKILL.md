---
name: code-challenge
description: Challenge a planned or finished change's decisions the way a senior reviewer would
argument-hint: "[plan or report path | ticket or doc URL | PR number | branch | nothing]"
allowed-tools:
  - Bash(gh pr diff *)
  - Bash(gh pr view *)
  - Bash(glab mr diff *)
  - Bash(glab mr view *)
  - Bash(git log *)
  - Bash(kit adrs *)
  - Bash(kit code-graph *)
  - Bash(kit git-base *)
  - Bash(kit scratch-dir *)
---

# /code-challenge

Challenge a proposed or finished change the way a senior reviewer would, decision first and code second. Most useful before the change is written, on your own plan or on someone else's design doc.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                                           | Read                                           |
| ------------------------------------------------ | ---------------------------------------------- |
| Filtering findings, or choosing the verdict line | [reference/standard.md](reference/standard.md) |

## Inputs

Resolve the artifact, first match wins:

1. A file path: a plan or a report.
2. A ticket URL: resolved as `investigate` step 0 describes.
3. A design doc or RFC URL: fetched through the docs connector (`skills/kit-rules/reference/tooling.md` section 4), with its comments.
4. A PR number or branch: its title, body, changed files, and diff, through the code host per `skills/kit-rules/reference/tooling.md` section 4.
5. Nothing: the current plan file in plan mode, else the newest report in `kit scratch-dir`, else `kit git-base --diff`.

If the artifact links a ticket, fetch that too; the "why" lives there. Read the project's CLAUDE.md and load the pattern skills it suggests.

ADRs: `kit adrs <path>` for each path the artifact touches, and `kit adrs` alone, searched with `rg` for its terms. Each one read feeds checks 2 and 4. `kit adrs` failing means the repo keeps none.

Mode follows the input:

- **Pre-change** (plan, report, ticket): check against the files and symbols the artifact names, read from the repo.
- **Post-change** (diff, PR): check against the diff.
- **Doc review** (someone else's design doc): pre-change, plus three checks of the doc itself. Every claim it makes about the code (a module's role, a call path, a count) is checked against the repo. Each assumption it leaves unstated, the one that has to hold for the proposal to work, is named. Each failure mode it misses, along `skills/kit-rules/reference/verify.md` section 2's axes, is listed. These come before the decision pass's own findings.

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
- Each finding's location is a `file:line` post-change, or the artifact's section or step pre-change and in a doc review.
- Each finding is shaped and worded per `skills/kit-rules/reference/output.md` section 4.

## Reply

The report path and the verdict line. On a doc review, then ask with AskUserQuestion whether to post the findings as comments on the doc.

## Post (doc review, after the user's yes)

One comment per finding, through the docs connector, anchored to the section it's about, worded per `skills/kit-rules/reference/output.md` section 4, in report order. Log each comment's link in the report under its finding. A finding the user dropped isn't posted.

## Stop

Read-only on the repo. Never edit, never comment on a PR or ticket, never approve. A doc gets comments only after the user's yes.

Arguments: `$ARGUMENTS`
