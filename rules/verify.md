# Verify

What must happen before a change is called done, and when. This file is the one definition of "done" and "verify"; others point here.

| When                                                                                                                 | What runs                                                                                                                                                                                                                                                         |
| -------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| After every edit                                                                                                     | The Stop hook's checks on the edited files. Automatic; a failure blocks the turn                                                                                                                                                                                  |
| Calling an ordinary change done                                                                                      | The refactor pass first: first green is never done (`skills/engineering-fundamentals/reference/red-green-refactor.md`). Then nothing more. Name in one line what the refactor pass changed and anything not run; the Stop hook shows its own run (next paragraph) |
| End of a plan, or when the user asks                                                                                 | `/code-review` (section 1); its hook runs the full gate                                                                                                                                                                                                           |
| Same moment, when the change has observable behavior (a response, output, an exit code, data written, a side effect) | The runtime pass (section 2)                                                                                                                                                                                                                                      |

**Hooks run the gates and show their results; Claude does neither.** The Stop hook checks a turn's edits (`stop checks:`). The `/code-review` hook runs the full gate (`run-checks checks:`), and Stop skips a tree it already checked. Each shows the user every PASS, FAIL, and SKIP line. Claude never runs `kit run-checks` (the bash guard blocks it; `--plan` only lists the gates). A reply never repeats verdict lines or counts: it acts on a FAIL, or says what wasn't run. A targeted test while developing is fine.

Tests and type checks prove the code compiles and the asserted paths pass. They don't prove the change works for whoever consumes it; that is what the runtime pass is for. A pure refactor with no behavior change is covered by the existing tests.

## 1. Self-review (`/code-review`)

Read your own diff as a reviewer who didn't write it and doesn't trust it. When it first stops, a SubagentStop hook runs the full gate on the checkout and sends it back with any FAIL. A PR reviewed in a worktree is gated by its CI (`gh pr checks`), not locally. Ticking a plan's last step blocks the stop until a review started after the last edit has finished.

The reviewer's report follows these, whoever prompted the checks:

- It never lists the checks or their counts: the hook already showed the user every line. For a worktree review, it opens with the PR's CI state from `gh pr checks`. Only the branch-left line from `rules/workflow.md` section 1 goes above it.
- A FAIL becomes a finding only once its cause is verified in the code and you're at least 90% sure, and never twice for something already reported.
- A FAIL left unreported gets one line saying why: pre-existing, flaky, or unrelated.

A PR or another branch is reviewed checked out, never from its diff alone; this applies to `/verify` too. How it's checked out: `rules/workflow.md` section 1.

- Every new helper, module, type, or abstraction names what was searched for and not found, per `rules/change.md` section 2. "I didn't look" is a finding against the change.
- Every non-obvious decision gets one line answering "why this, not the obvious alternative". A decision you can't justify is a decision you took from a generated draft without checking it.
- What a refactor pass would have removed is a finding: duplication, a redundant call, a leftover, a helper restating another. Code committed at first green skipped that pass.
- Review findings are claims, per `rules/workflow.md` section 4.

## 2. Use it until it breaks (the runtime pass, with `/verify`)

Exercise the change through the interface its consumers use, the way they use it:

| Surface   | Exercise it by                                                     |
| --------- | ------------------------------------------------------------------ |
| UI        | driving the affected flow in the running app                       |
| API       | real HTTP calls against a running server                           |
| CLI       | invoking the built binary or script, checking output and exit code |
| Library   | a scratch caller importing the public API                          |
| Migration | running it on a copy of realistic data                             |
| Infra     | `plan` or a dry run against the real target                        |

Starting it and seeing it respond is not verification. The goal is to break it.

Exercise every row that applies to the change:

| Axis       | Cases                                                                                                                    |
| ---------- | ------------------------------------------------------------------------------------------------------------------------ |
| Input      | empty, exactly one, many (past a page or batch), boundary and oversized values, malformed input, missing optional fields |
| Failure    | dependency errors, timeout or slow response, partial failure mid-operation, insufficient permission                      |
| Repetition | retry or double submit (idempotency), concurrent writers, interrupted mid-run then rerun                                 |
| State      | stale cache or data after a write, existing data in the old shape, rollback                                              |
| UI only    | loading state, keyboard only, narrow viewport, back or refresh mid-flow                                                  |
| CLI only   | no args and `--help`, invalid flags, piped stdin and non-TTY output, stdout vs stderr, exit codes, Ctrl-C mid-run        |

The UI and CLI rows are additive: they apply only when the change touches that surface, and a change that doesn't never lists them as Not exercised.

When the setup can't produce a case (no fixture with many items, no limited-permission user), say so; don't skip it silently.

The report lists three things, one line per case:

1. **Exercised**: the case and what happened.
2. **Not exercised**: the case and why (no data, no access, out of scope).
3. **Raise with**: see section 3.

"Looked fine" is not a result. A case that broke goes back to the fix loop when section 5 says it's a regression; anything else is a follow-up.

## 3. Raise upstream gaps

An edge case the spec, the contract, or the design doesn't cover is a question for the person who owns it, not something to patch around quietly.

- List each under **Raise with** naming the owner of the contract (upstream service, API, schema, design, product) and the concrete case: input, observed behavior, what's undefined.
- A workaround on your side of the boundary is allowed only when named in the change and in the Raise with list. An unnamed one is a defect.

## 4. The human orchestrates

The agent proposes; the human decides and owns the result. Agent output, including a generated plan, review, or fix, is a draft with its reasons attached, never a decision already taken.

- Present a non-obvious choice with its alternative and the reason, so the human can overrule it.
- Don't claim done on behalf of the human. Done means the checks the table at the top requires ran after the last edit, with their output cited; without that, say what was not run.

## 5. One cycle, then stop

A change gets one `/code-review` and one runtime pass. A finding from either goes back to the fix loop only when it's a regression: the change leaves something worse than on the base branch (`kit git-base`). That means a new failure, a path that worked and now breaks, or a guard or check weakened.

Everything else is a follow-up line in the report and the PR description, never another cycle: a pre-existing problem, an edge case the base already had, a hardening idea, a gap in new code that still improves on the base.

- Fixing a regression re-runs only the case or check that found it, not the whole review or runtime pass.
- Another full review or runtime pass runs only when the user asks for one.
