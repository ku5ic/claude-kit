---
name: code-deep-review
description: A deep review of a path or change: design, precedent, test strength, error paths, and security
argument-hint: "<path | PR number | branch | nothing for the branch's diff>"
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

# /code-deep-review

A deep review of a path or a change, run now and then, covering what the per-change `/code-review` skips: design, precedent, test strength, error paths, and security. With nothing and an empty diff, ask for a path before anything else.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before             | Read                                             |
| ------------------ | ------------------------------------------------ |
| Running the passes | [reference/checklist.md](reference/checklist.md) |

## Inputs

1. Resolve the target, first match wins:
   - A path: every tracked file under it.
   - A PR number or branch is checked out where a trailing `here` or `worktree` says, or where the user answers when asked, per `skills/kit-rules/reference/workflow.md` section 1. The target is its diff against the base.
   - Nothing: `kit git-base --diff`.
2. For a diff, the review covers the changed lines and the code they call or are called from, never the whole files.
3. Read the project's CLAUDE.md, and name the pattern skill of every stack the target holds. An unsplit run loads what the Check against cells name.

## Passes

Run each pass, fanning out per `skills/kit-rules/reference/subagents.md` section 4. Design is a Judge part on every target, a diff included, in its own agent. A pass with nothing to report is left out of the report.

| Pass                       | Check against                                                                                                                                                                                         |
| -------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Design                     | `engineering-fundamentals`: `design-integrity.md`, and `metric-thresholds.md` for size, nesting, and coupling. its [reference/checklist.md](reference/checklist.md) Design bullet                     |
| Stack                      | the `anti-patterns.md` of each pattern skill Inputs step 3 names                                                                                                                                      |
| Tests                      | `test-patterns`. For each changed behavior, name the assertion that fails without it. None found is a finding                                                                                         |
| Precedent                  | how the repo already solves the same thing, per `skills/kit-rules/reference/change.md` section 2. A second way of doing a solved thing is a finding                                                   |
| Errors and observability   | the target alone: an error swallowed, logged and dropped, or rethrown without its cause; a failure with no log or metric to find it by; a log line missing the identifier needed to trace one request |
| Security                   | `security-patterns`: its `anti-patterns.md` and the reference for each surface the target touches                                                                                                     |
| Functionality              | its [reference/checklist.md](reference/checklist.md) Functionality bullet                                                                                                                             |
| Documentation and comments | its [reference/checklist.md](reference/checklist.md) Documentation and Comments bullets                                                                                                               |

Every pass also follows the checklist's bullets on reading every human-written line, naming a part it can't judge, saying what a partial review covered, and naming a good practice, and returns what they ask for with its candidates. Every finding cites `file:line` and what was read there. A finding inferred without reading the code it names is labelled per `skills/kit-rules/reference/evidence.md` section 1.

## Report

Load the report-format skill and use its format, written to the path `kit scratch-dir review <target-slug>` prints. The Scope line names the target and, for a diff, its base. Each finding's heading names its pass after the severity (`warning: Tests, ...`).

## Reply

The report path, the counts by severity, and the first failure, if any.

## Out of scope

- Running the checks, which the per-change `/code-review` gate does.
- Fixing anything. Findings go back as instructions.
- Whether the change should exist at all, which `/code-challenge` asks.

Arguments: `$ARGUMENTS`
