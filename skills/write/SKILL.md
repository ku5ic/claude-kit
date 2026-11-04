---
name: write
description: Draft outward-facing text (commit message, release notes, devnote, explainer, stakeholder summary). Use when the user asks for one of these.
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

1. Missing or unknown kind: ask, with the likeliest kinds as options.
2. Read the kind's reference file and follow it as the procedure.

<!-- chat: skip -->

3. In the reference, the `ARGUMENTS` token (written with a leading dollar sign) means everything after the kind word in the Arguments line below, and a backticked command prefixed with an exclamation mark means run that command via Bash and use its output.

<!-- chat: end -->

<!-- chat: only
In claude.ai chat, the arguments are the user's message after the kind word, and the material is what they pasted. Nothing can be run: if the diff, log, or text a kind needs isn't in the conversation, ask for it.
-->

4. Every kind follows `rules/output.md` section 4 (External communication). On tone and length it wins over the reference file; the sections a reference file defines, such as the release-notes sections, stay.

<!-- chat: skip -->

Arguments: `$ARGUMENTS`

<!-- chat: end -->
