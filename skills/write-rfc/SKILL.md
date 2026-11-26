---
name: write-rfc
description: A design doc or RFC whose current state, rollout, rollback, and capacity come from the code and real metrics
argument-hint: "<the proposal, or a thread, ticket, or doc URL>"
allowed-tools:
  - Bash(git log *)
  - Bash(git show *)
  - Bash(kit adrs *)
  - Bash(kit code-graph *)
  - Bash(kit scratch-dir *)
---

# /write-rfc

A design doc for a change not yet made: the system as it is, the proposal, how it ships and how it's undone, and what load it has to carry. Every claim about the system comes from the code or a measurement.

Tone and shape: `skills/kit-rules/reference/output.md` section 4, which wins over this file; the sections this file defines stay.

<!-- chat: only
In claude.ai chat, the arguments are the user's message and the material is what they pasted. Nothing can be run: the current state and the numbers come from the paste, and a section with no material says what it needs.
-->

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                                                           | Read                                                                 |
| ---------------------------------------------------------------- | -------------------------------------------------------------------- |
| Choosing the sections, or writing Rollout, Rollback, or Capacity | [reference/design-doc-practice.md](reference/design-doc-practice.md) |

## Where the facts come from

<!-- chat: skip -->

- A linked thread, ticket, or doc is resolved as `skills/investigate` step 0 describes. The project's own RFC or design-doc template, when its CLAUDE.md, CONTRIBUTING, or docs folder has one, replaces the sections below.
- The architecture model is in the folder `kit code-graph model-dir` prints: its `README.md`, and the `c4-*.mmd` views covering what the proposal touches. No model, or `kit code-graph check-c4` lists problems: run the code-visual skill's procedure on the touched paths first.
- `kit blast-radius <file>` gives each changed file's consumers, and `kit adrs <path>` the decisions already governing it.
- Numbers come from the monitoring connector (`skills/kit-rules/reference/tooling.md` section 4): the touched services' traffic, latency, error rate, and saturation over the last 30 days.
- Failure modes follow `skills/kit-rules/reference/verify.md` section 2's axes.

<!-- chat: end -->

<!-- chat: only
In claude.ai chat, the facts come from the paste: the code, the diagrams, and the numbers the user gave.
-->

## Procedure

1. Source: the arguments, and the thread, ticket, or doc they link, read whole.
2. Current state: the containers, modules, and consumers the proposal touches, and the decisions governing them. Each statement cites a `file:line`, a diagram, or an ADR.
3. Proposal: what changes, at the level of the current state, with its own diagram when a container or an edge changes.
4. Options: at least two, one of them the cheapest that could work (`skills/kit-rules/reference/change.md` section 7), each with its cost and what it gives.
5. Rollout: the order of steps, what's behind a flag, how each step is checked, and who's told.
6. Rollback: for each rollout step, how it's undone and what can't be (a migration, sent data), with the point of no return named.
7. Capacity: the touched services' current load, each number with its query and window. A number with no source goes under Open questions as a measurement to take, never an estimate written as measured.
8. Risks and failure modes: each dependency's failure, and the inputs and load that break the proposal.
9. Open questions: what the sources couldn't settle, each with who or what could.

<!-- chat: skip -->

## Output file

The project's design-doc folder, when it has one; else the path `kit scratch-dir rfc <slug>` prints. A docs tool the user names gets the doc through its connector, after their yes. Reply with the path, the Options line that wins, and the open-question count.

<!-- chat: end -->

<!-- chat: only
In claude.ai chat, print the doc in the reply, then the open-question count.
-->

<!-- chat: skip -->

Arguments: `$ARGUMENTS`

<!-- chat: end -->
