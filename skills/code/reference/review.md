# /code review

A deep review of a path or a change, run now and then, covering what the per-change `/code-review` skips: design, precedent, test strength, error paths, and security. Arguments: `<path | PR number | branch | nothing for the branch's diff>`. With nothing and an empty diff, ask for a path before anything else.

## Inputs

1. Resolve the target, first match wins:
   - A path: every tracked file under it.
   - A PR number or branch is checked out where a trailing `here` or `worktree` says, or where the user answers when asked, per `skills/kit-rules/reference/workflow.md` section 1. The target is its diff against the base.
   - Nothing: `kit git-base --diff`.
2. For a diff, the review covers the changed lines and the code they call or are called from, never the whole files.
3. Read the project's CLAUDE.md. Load `engineering-fundamentals`, `test-patterns`, `security-patterns`, and every pattern skill the `<suggested-skills>` block or the touched files call for.

## Passes

Run each pass over the target. A pass with nothing to report is left out of the report.

| Pass                     | Check against                                                                                                                                                                       |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Design                   | `engineering-fundamentals`: `design-integrity.md`, and `metric-thresholds.md` for size, nesting, and coupling                                                                       |
| Stack                    | each loaded pattern skill's `anti-patterns.md`                                                                                                                                      |
| Tests                    | `test-patterns`. For each changed behavior, name the assertion that fails without it. None found is a finding                                                                       |
| Precedent                | how the repo already solves the same thing, per `skills/kit-rules/reference/change.md` section 2. A second way of doing a solved thing is a finding                                 |
| Errors and observability | an error swallowed, logged and dropped, or rethrown without its cause; a failure with no log or metric to find it by; a log line missing the identifier needed to trace one request |
| Security                 | `security-patterns`: its `anti-patterns.md` and the reference for each surface the target touches                                                                                   |

Every finding cites `file:line` and what was read there. A finding inferred without reading the code it names is labelled per `skills/kit-rules/reference/evidence.md` section 1.

## Report

Load the report-format skill and use its format, written to the path `kit scratch-dir review <target-slug>` prints. The Scope line names the target and, for a diff, its base. Each finding's heading names its pass after the severity (`warning: Tests, ...`).

## Reply

The report path, the counts by severity, and the first failure, if any.

## Out of scope

- Running the checks, which the per-change `/code-review` gate does.
- Fixing anything. Findings go back as instructions.
- Whether the change should exist at all, which `/code challenge` asks.
