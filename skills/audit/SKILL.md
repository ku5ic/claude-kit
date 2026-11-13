---
name: audit
description: Targeted read-only audit of a code surface (a11y, debt, doc-drift, perf, security, ci, infra, slop), or re-verify a prior audit report
argument-hint: <a11y|debt|doc-drift|perf|security|ci|infra|slop|verify> <target, or a link to an external tracker/doc>
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
---

## Dispatch

The first word of the arguments is the kind; everything after it is the kind's arguments.

| Kind        | Procedure                                        |
| ----------- | ------------------------------------------------ |
| `a11y`      | [reference/a11y.md](reference/a11y.md)           |
| `ci`        | [reference/ci.md](reference/ci.md)               |
| `debt`      | [reference/debt.md](reference/debt.md)           |
| `doc-drift` | [reference/doc-drift.md](reference/doc-drift.md) |
| `infra`     | [reference/infra.md](reference/infra.md)         |
| `perf`      | [reference/perf.md](reference/perf.md)           |
| `security`  | [reference/security.md](reference/security.md)   |
| `slop`      | [reference/slop.md](reference/slop.md)           |
| `verify`    | [reference/verify.md](reference/verify.md)       |

A diff's security review is the built-in `/security-review`; a tree's is `security`. Simplification is the built-in `/simplify <path>`, which reviews a path beyond the diff.

1. If the kind is missing or unknown, ask per `skills/kit-rules/reference/workflow.md` section 4, in groups: code (`debt`, `perf`, `security`), surface (`a11y`, `ci`, `infra`), prose (`doc-drift`, `slop`), and re-check (`verify`).
2. Read the kind's reference file. With no argument and an Arguments line that offers no `none`, ask for the target before dispatching, since the auditor can't ask.
3. For every kind but `verify`: resolve external context as `investigate` step 0 describes, then dispatch the auditor agent (subagent_type: kit:auditor, foreground) with the reference's steps and the resolved arguments; it writes the report, you reply per `skills/kit-rules/reference/output.md` section 3. For `verify`, follow the reference yourself. It dispatches the auditor with its own finding list.
4. In the reference, the `ARGUMENTS` token (written with a leading dollar sign) means everything after the kind word in the Arguments line below, and a backticked command prefixed with an exclamation mark means run that command via Bash and use its output.

Arguments: `$ARGUMENTS`
