---
name: code
description: Understand or challenge code, read-only - a path's git history (history), Mermaid diagrams (visual), what a change breaks (impact), an onboarding walk (tour), or a change's decisions (challenge)
argument-hint: <history|visual|impact|tour|challenge> <path, component name, plan, PR, or nothing>
disable-model-invocation: true
allowed-tools:
  - Bash(gh pr diff *)
  - Bash(gh pr view *)
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
| `tour`      | [reference/tour.md](reference/tour.md)           |
| `visual`    | [reference/visual.md](reference/visual.md)       |

1. If the kind is missing or unknown, ask via AskUserQuestion with the kinds above as options.
2. Read the kind's reference file and follow it yourself.
3. Each kind writes its report to the path `kit scratch-dir <kind> <slug>` prints; the reply is the one its Reply section names, per `skills/kit-rules/reference/output.md` section 3.
4. Read-only on the repo: a kind writes only its scratch report, its raw folder, and the paths its reference names. It never posts or comments anywhere.

Arguments: `$ARGUMENTS`
