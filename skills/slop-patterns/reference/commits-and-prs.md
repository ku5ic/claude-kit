# Commits, PR text, review replies

The rules for these are `skills/kit-rules/reference/output.md` section 4 and, for a commit, `skills/write/reference/commit.md`. This file covers the tells in commit messages and PR text.

The pattern to avoid is the edit summary a model writes when asked to describe a diff it didn't reason about: long, procedural, reassuring, and unspecific. A human summary is short, names the thing, and cites the reason.

| Tell                                                    | Rule                 | Why it's empty                                                    | Instead                                             |
| ------------------------------------------------------- | -------------------- | ----------------------------------------------------------------- | --------------------------------------------------- |
| A generic reason: readability, maintainability, clarity | `generic-reason`     | Every change claims it                                            | What changed: "split the 300-line switch"           |
| "Streamlined"                                           | `streamlined`        | Says less code without saying which                               | What was removed or merged                          |
| "While preserving existing behavior"                    | `while-preserving`   | Reassurance about what didn't change; the tests and diff say that | Cut it. A line naming a check that wasn't run stays |
| "Addressed review feedback"                             | `addressed-feedback` | Restates why the commit exists                                    | What the feedback changed                           |
| An "-ing" tail: ", ensuring ..."                        | `ing-tail`           | Claims an effect without showing it                               | The effect as a fact, or cut                        |
| A bulleted list of every file touched                   | none, judgment       | The diff already lists them                                       | The reason the files moved together                 |

The more improvements stack up in one summary, the stronger the signal. One concrete change beats four adjectives.

`generic-reason`, `streamlined`, `addressed-feedback`, and `commit-opener` are house style, not sourced tells: no study names them, and pre-2022 human commits use "address PR comments" too. They block because each tells a reviewer nothing, whoever wrote it. `while-preserving` is sourced: Wikipedia lists "preserved" and "retained" information among AI edit-summary tells.

## Subjects

A subject or body line that opens with "Certainly", "Here is", "This commit" and similar is blocked (`commit-opener`).
