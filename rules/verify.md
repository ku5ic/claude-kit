# Verify

What must happen before a change is called done, and when. This file is the one definition of "done" and "verify"; others point here.

| When                                                                                                                 | What runs                                                                        |
| -------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| After every edit                                                                                                     | The Stop hook's checks on the edited files. Automatic; a failure blocks the turn |
| Calling an ordinary change done                                                                                      | Nothing more. Cite the Stop hook's result, and name in one line anything not run |
| End of a plan, or when the user asks                                                                                 | The full `kit run-checks` suite and self-review (section 1)                      |
| Same moment, when the change has observable behavior (a response, output, an exit code, data written, a side effect) | The runtime pass (section 2)                                                     |

Tests and type checks prove the code compiles and the asserted paths pass. They don't prove the change works for whoever consumes it; that is what the runtime pass is for. A pure refactor with no behavior change is covered by the existing tests.

## 1. Self-review (`/code-review`)

Read your own diff as a reviewer who didn't write it and doesn't trust it.

- Every new helper, module, type, or abstraction names what was searched for and not found, per `rules/change.md` section 2. "I didn't look" is a finding against the change.
- Every non-obvious decision gets one line answering "why this, not the obvious alternative". A decision you can't justify is a decision you took from a generated draft without checking it.
- Review findings are claims, per `rules/workflow.md` section 4. Check each against the code before applying it.

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

"Looked fine" is not a result. A case that broke goes back to the fix loop before anything else.

## 3. Raise upstream gaps

An edge case the spec, the contract, or the design doesn't cover is a question for the person who owns it, not something to patch around quietly.

- List each under **Raise with** naming the owner of the contract (upstream service, API, schema, design, product) and the concrete case: input, observed behavior, what's undefined.
- A workaround on your side of the boundary is allowed only when named in the change and in the Raise with list. An unnamed one is a defect.

## 4. The human orchestrates

The agent proposes; the human decides and owns the result. Agent output, including a generated plan, review, or fix, is a draft with its reasons attached, never a decision already taken.

- Present a non-obvious choice with its alternative and the reason, so the human can overrule it.
- Don't claim done on behalf of the human. Done means the checks the table at the top requires ran after the last edit, with output cited, per `rules/evidence.md` section 1.
