---
name: write-adr
description: An architecture decision record in the project's own format, every sentence traced to a source
argument-hint: "<the decision, or a thread, ticket, or doc URL> [supersedes: <number>]"
allowed-tools:
  - Bash(git log *)
  - Bash(git show *)
  - Bash(kit adrs *)
  - Bash(kit project-root)
---

# /write-adr

Record an architecture decision the way the project records them, with options that include the cheapest one and a source for every sentence.

Tone and shape: `skills/kit-rules/reference/output.md` section 4, which wins over this file; the sections this file defines stay.

<!-- chat: only
In claude.ai chat, the arguments are the user's message and the material is what they pasted. Nothing can be run: if the thread, the code, or an earlier ADR this needs isn't in the conversation, ask for it.
-->

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                                            | Read                                                   |
| ------------------------------------------------- | ------------------------------------------------------ |
| Drafting the ADR, or deciding whether one is owed | [reference/adr-practice.md](reference/adr-practice.md) |

## The project's rules first

<!-- chat: skip -->

1. Read the project's own ADR rules: its CLAUDE.md and CONTRIBUTING (when to write one, numbering, amend or supersede, which date), and the ADR folder. `kit adrs` lists the accepted ones, and fails when the project has no ADR folder.
2. The template, first match wins: `templates/template.md` in the ADR folder (adr-tools), `adr-template.md` there (MADR), else the headings of the newest ADR.

<!-- chat: end -->

The project's rules win. With none, this bar holds:

- An ADR records an architecture decision, or a change in what the system does for its users. A fix, a tuning, or a refactor gets none: say so, and stop.
- A decision that replaces an accepted one gets a new ADR that supersedes it. An accepted ADR changes only in its status line.
- The status is Proposed until the user says the decision is accepted.

## Procedure

<!-- chat: skip -->

A linked thread, ticket, or doc is resolved as `skills/investigate` step 0 describes. `kit adrs <path>` lists the accepted ADRs on each path the decision touches.

<!-- chat: end -->

1. Source: the arguments, and the thread, ticket, or doc they link, read whole.
2. Precedent: the accepted ADRs on the paths the decision touches, each read. One this decision contradicts is superseded by it, and Context says so.
3. Context: the forces behind the decision. Each sentence traces to a commit, a PR or thread, a doc, or the code, and Context names the source. A sentence with no source gets asked about or cut (`skills/kit-rules/reference/evidence.md` section 1).
4. Options: at least two, one of them the cheapest that could work (`skills/kit-rules/reference/change.md` section 7), each with what it costs and what it gives, from the sources. A template with no options section takes them in Context.
5. Decision: the chosen option in one paragraph, active voice ("We use X"). Paths and folders it governs go in backticks, so a later edit of them finds this ADR.
6. Consequences: what gets easier, what gets harder, and what's owed now.

<!-- chat: skip -->

## Output file

- With an `.adr-dir` and `adr` on PATH: `VISUAL=true adr new <title>`, with `-s <n>` for each ADR it supersedes, then fill the file it prints with Edit.
- Otherwise: the next number in the ADR folder, named the way its files are. With no ADR folder, ask where it goes.
- The date follows the project's rule, else today.

Reply with the path, and one line per sentence still lacking a source.

<!-- chat: end -->

<!-- chat: only
In claude.ai chat, print the ADR in the reply, in the template's shape, then one line per sentence still lacking a source.
-->

<!-- chat: skip -->

Arguments: `$ARGUMENTS`

<!-- chat: end -->
