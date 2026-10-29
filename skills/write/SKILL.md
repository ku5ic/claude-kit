---
name: write
description: Draft outward-facing text - commit message, release notes, devnote, explainer, stakeholder summary
argument-hint: <commit|release-notes|devnote|explainer|stakeholder> [kind arguments]
disable-model-invocation: true
allowed-tools:
  - Bash(git status *)
  - Bash(git diff *)
  - Bash(git log *)
  - Bash(git branch --show-current)
  - Bash(git rev-parse *)
---

## Dispatch

The first word of the arguments is the kind; everything after it is the kind's arguments.

| Kind            | Procedure                                                |
| --------------- | -------------------------------------------------------- |
| `commit`        | [reference/commit.md](reference/commit.md)               |
| `release-notes` | [reference/release-notes.md](reference/release-notes.md) |
| `devnote`       | [reference/devnote.md](reference/devnote.md)             |
| `explainer`     | [reference/explainer.md](reference/explainer.md)         |
| `stakeholder`   | [reference/stakeholder.md](reference/stakeholder.md)     |

1. Missing or unknown kind: ask via AskUserQuestion with the likeliest kinds as options.
2. Read the kind's reference file and follow it as the procedure.
3. In the reference, the `ARGUMENTS` token (written with a leading dollar sign) means everything after the kind word in the Arguments line below, and a backticked command prefixed with an exclamation mark means run that command via Bash and use its output.
4. Every kind follows `rules/output.md` section 4 (External communication). On tone and length it wins over the reference file; the sections a reference file defines, such as the release-notes sections, stay.

Arguments: `$ARGUMENTS`
