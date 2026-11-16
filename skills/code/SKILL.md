---
name: code
description: Read-only code insight - git history (history), Mermaid diagrams (visual), what a change breaks (impact), an onboarding walk (tour), a change's decisions (challenge), or deep review (review)
argument-hint: <history|visual|impact|tour|challenge|review> <path, component name, plan, PR, or nothing> [here|worktree]
disable-model-invocation: true
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

## Dispatch

The first word of the arguments is the kind; everything after it is the kind's arguments.

| Kind        | Procedure                                        |
| ----------- | ------------------------------------------------ |
| `challenge` | [reference/challenge.md](reference/challenge.md) |
| `history`   | [reference/history.md](reference/history.md)     |
| `impact`    | [reference/impact.md](reference/impact.md)       |
| `review`    | [reference/review.md](reference/review.md)       |
| `tour`      | [reference/tour.md](reference/tour.md)           |
| `visual`    | [reference/visual.md](reference/visual.md)       |

1. If the kind is missing or unknown, ask per `skills/kit-rules/reference/workflow.md` section 4, in groups: understand (`history`, `visual`, `impact`, `tour`) and judge (`challenge`, `review`).
2. Read the kind's reference file and follow it yourself.
3. Each kind writes its report to the path `kit scratch-dir <kind> <slug>` prints; the reply is the one its Reply section names, per `skills/kit-rules/reference/output.md` section 3.
4. Read-only on the repo: a kind writes only its scratch report, its raw folder, and the paths its reference names. The one exception is `review` of a PR or branch, which checks it out. No kind ever posts or comments anywhere.

Arguments: `$ARGUMENTS`
