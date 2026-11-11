# Verify

What must happen before a change is called done. This file is the one definition of "done" and "verify".

- After an edit, run nothing. The Stop hook checks the edited files and shows its receipt.
- Before calling an ordinary change done, do the refactor pass (`skills/engineering-fundamentals/reference/red-green-refactor.md`), then name in one line what it changed and what wasn't run.
- At the end of a plan, or when the user asks, run `/code-review` (section 1). Its hook runs the full gate.
- At the same moment, a change with observable behavior (a response, output, an exit code, data written, a side effect) gets the runtime pass (section 2).
- A gate verdict is never repeated in a reply. Act on a FAIL, or say what wasn't run.
- A targeted test while developing is fine.
- A pure refactor that doesn't touch UI is covered by the existing tests.

## 1. Self-review (`/code-review`)

- Read your own diff as a reviewer who didn't write it.
- A PR or another branch gets reviewed checked out, per `skills/kit-rules/reference/workflow.md` section 1. In a worktree, its gate is the PR's CI (`gh pr checks`). This applies to `/verify` too.
- The report never lists the checks or their counts. A worktree review opens with the PR's CI state, below only the branch-left line.
- A FAIL becomes a finding once its cause is verified in the code at 90% or more, and only once. A FAIL left out gets one line, pre-existing, flaky, or unrelated.
- A new helper, module, type, or abstraction names what was searched for and not found.
- A non-obvious decision gets one line on why not the obvious alternative.
- Whatever a refactor pass would have removed (duplication, a redundant call, a leftover) is a finding.

## 2. Use it until it breaks (the runtime pass, with `/verify`)

Exercise the change through the interface its consumers use:

| Surface   | Exercise it by                                                     |
| --------- | ------------------------------------------------------------------ |
| UI        | driving the affected flow in the running app                       |
| API       | real HTTP calls against a running server                           |
| CLI       | invoking the built binary or script, checking output and exit code |
| Library   | a scratch caller importing the public API                          |
| Migration | running it on a copy of realistic data                             |
| Infra     | `plan` or a dry run against the real target                        |

- A plan step that changes what a user sees gets this pass before its pause.
- A change whose contract is "unchanged" (a refactor, migration, or rewire) runs the same flow on the base branch (`kit git-base`) and on the change.
- Try to break it, on every axis that applies:

| Axis       | Cases                                                                                                                    |
| ---------- | ------------------------------------------------------------------------------------------------------------------------ |
| Input      | empty, exactly one, many (past a page or batch), boundary and oversized values, malformed input, missing optional fields |
| Failure    | dependency errors, timeout or slow response, partial failure mid-operation, insufficient permission                      |
| Repetition | retry or double submit (idempotency), concurrent writers, interrupted mid-run then rerun                                 |
| State      | stale cache or data after a write, existing data in the old shape, rollback                                              |
| UI only    | loading state, keyboard only, narrow viewport, back or refresh mid-flow                                                  |
| CLI only   | no args and `--help`, invalid flags, piped stdin and non-TTY output, stdout vs stderr, exit codes, Ctrl-C mid-run        |

- The UI and CLI rows apply only to a change touching that surface.
- A case the setup can't produce goes under Not exercised, with the reason.
- The report gives one line per case under **Exercised** (what happened), **Not exercised** (why), and **Raise with** (section 3).

## 3. Raise upstream gaps

- A case the spec, contract, or design leaves undefined goes under Raise with, naming the owner and the input, the observed behavior, and what's undefined.
- A workaround on your side of the boundary is allowed only when named in the change and in Raise with.

## 4. The human orchestrates

- A non-obvious choice is presented with its alternative and the reason, so the human can overrule it.
- Done means the checks this file requires ran after the last edit, with their output cited. Otherwise, say what wasn't run.

## 5. One cycle, then stop

- A change gets one `/code-review` and one runtime pass.
- A finding goes back to the fix loop only when it's a regression against the base branch (`kit git-base`), meaning a new failure, a path that worked and now breaks, or a weakened check.
- Anything else is a follow-up line in the report and the PR description.
- Fixing a regression re-runs only the case or check that found it.
- Another full review or runtime pass runs only when the user asks.
