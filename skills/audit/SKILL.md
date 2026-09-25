---
description: Targeted read-only audit of a code surface - a11y, debt, doc-drift, perf - or re-verify a prior audit report
argument-hint: <a11y|debt|doc-drift|perf|verify> <target, or a link to an external tracker/doc>
disable-model-invocation: true
---

## Dispatch

The first word of the arguments is the kind; everything after it is the kind's arguments.

| Kind        | Procedure                                          |
| ----------- | -------------------------------------------------- |
| `a11y`      | [references/a11y.md](references/a11y.md)           |
| `debt`      | [references/debt.md](references/debt.md)           |
| `doc-drift` | [references/doc-drift.md](references/doc-drift.md) |
| `perf`      | [references/perf.md](references/perf.md)           |
| `verify`    | [references/verify.md](references/verify.md)       |

Security audits use the built-in `/security-review`.

1. Missing or unknown kind: ask via AskUserQuestion with the kinds above as options.
2. Read the kind's reference file and follow it as the procedure.
3. In the reference, the `ARGUMENTS` token (written with a leading dollar sign) means everything after the kind word in the Arguments line below, and a backticked command prefixed with an exclamation mark means run that command via Bash and use its output.

Arguments: `$ARGUMENTS`
