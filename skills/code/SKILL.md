---
name: code
description: Understand a codebase or a part of it, read-only - the git and PR history of a path (history), or a visual tour of Mermaid diagrams built from its size, imports, churn, and owners (visual)
argument-hint: <history|visual> <path, component name, or nothing for the whole repo>
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
  - Bash(kit code-graph *)
  - Bash(kit scratch-dir *)
---

## Dispatch

The first word of the arguments is the kind; everything after it is the kind's arguments.

| Kind      | Procedure                                    |
| --------- | -------------------------------------------- |
| `history` | [reference/history.md](reference/history.md) |
| `visual`  | [reference/visual.md](reference/visual.md)   |

1. If the kind is missing or unknown, ask via AskUserQuestion with the kinds above as options.
2. Read the kind's reference file and follow it yourself.
3. Each kind writes its report to the path `kit scratch-dir <kind> <slug>` prints; the reply is the one its Reply section names, per `skills/kit-rules/reference/output.md` section 3.
4. Read-only on the repo: a kind writes only its scratch report, its raw folder, and the paths its reference names.

Arguments: `$ARGUMENTS`
