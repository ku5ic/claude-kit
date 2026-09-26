---
description: Targeted read-only audit of a code surface - a11y, debt, doc-drift, perf - or re-verify a prior audit report
argument-hint: <a11y|debt|doc-drift|perf|verify> <target, or a link to an external tracker/doc>
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
  - Bash(a11y-check.sh *)
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
2. Read the kind's reference file.
3. For `a11y`, `debt`, `doc-drift`, and `perf`: resolve external context (`rules/workflow.md` section 4), then dispatch the auditor agent (subagent_type: auditor, foreground) with the reference's steps and the resolved arguments; it writes the report, you relay its summary. For `verify`, follow the reference yourself: it dispatches the auditor with its own finding list.
4. In the reference, the `ARGUMENTS` token (written with a leading dollar sign) means everything after the kind word in the Arguments line below, and a backticked command prefixed with an exclamation mark means run that command via Bash and use its output.

Arguments: `$ARGUMENTS`
