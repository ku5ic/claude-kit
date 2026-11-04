---
name: investigate
description: Read-only investigation. Use when the user asks how, why, or where something works in the code, or reports unexpected behavior without asking for a change. Ends at findings, never edits.
argument-hint: <a question, a symptom, or a link to an external ticket/doc>
disable-model-invocation: true
allowed-tools:
  - Bash(kit slop *)
  - Bash(git diff *)
user-invocable: false
---

## When to use this

- A question: how, why, or where something works.

### Question

<!-- chat: skip -->

2. Skim: project CLAUDE.md, `rg` for the names involved, read the matched sections.

<!-- chat: end -->

<!-- chat: only
In claude.ai chat, answer from the pasted code.
-->

3. Answer, each claim labelled per `rules/evidence.md` section 1.
