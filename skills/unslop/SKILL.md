---
name: unslop
description: Find and rewrite AI tells and filler in a file, a diff, or pasted text, then show what changed. Use when the user asks to unslop, clean up, or de-AI a piece of text.
argument-hint: <file...|diff|-> [in place]
disable-model-invocation: true
allowed-tools:
  - Bash(kit slop *)
  - Bash(kit scratch-dir *)
  - Bash(git diff *)
---

## Procedure

1. Load `slop-patterns`. It says how to rewrite; this skill only says what to run on.

<!-- chat: skip -->

2. Resolve the target from the arguments:
   - Files: each path.
   - `diff`: the files `git diff --name-only` lists. `kit slop` scans whole files, so keep only the hits on lines `git diff -U0 <file>` adds, and rewrite nothing else.
   - `-` or no argument: ask for the text, then treat it as one file.
3. Scan: `kit slop <files>`. For pasted text, `kit slop --surface <file|commit|pr|chat|ui> -` with the text on stdin, `ui` for UI code; ask which surface when it isn't obvious.

<!-- chat: end -->

<!-- chat: only
In claude.ai chat, the target is the text in the user's message. If there's none, ask for it. Run the scan before judging anything yourself: save the text to a file, then from this skill's folder run `sh scripts/scan.sh <file|commit|pr|chat|ui> < <that file>`, with `ui` for UI code, asking which surface when it isn't obvious. Exit 0 means nothing blocks and 1 means hits; both print what they found.

In claude.ai chat, any other exit, or no way to run it, means the scan didn't happen. Your next action is then to read `reference/lexicon.md` in this skill's folder. Take every hit and tier from that file, never from memory, and say in the reply that these are a hand read.
-->

4. Rewrite every tier 1 and 2 hit, and every passage the scan reports as dense, per `slop-patterns` `reference/rewrite.md`. Then read the whole text once for what no regexp sees. Look for sentences that carry nothing, sections that restate others (`slop-patterns` `reference/principles.md`), and every row marked "none, judgment" in `slop-patterns` `reference/content-and-grammar.md`. For UI files, also walk `slop-patterns` `reference/design.md` section 12.
5. Check the facts. List each factual claim the text makes about code, versions, commands, or another system. Verify each with a tool that reaches the source, or cut it, per `skills/kit-rules/reference/evidence.md` section 1. A rewrite that reads well but states something unchecked is still slop.

<!-- chat: skip -->

6. Write the result:
   - Default: a copy at the path `kit scratch-dir unslop <slug>` prints, one per file.
   - With `in place` in the arguments, edit the file itself.
7. Re-scan the result with `kit slop`. A hit left on purpose gets a line in the reply saying why.

## Output

Per `skills/kit-rules/reference/output.md` section 3: the path of each result, hits before and after per tier, and `git diff --no-index <original> <result>` for a copy. Nothing else.

Arguments: `$ARGUMENTS`

<!-- chat: end -->

<!-- chat: only
In claude.ai chat, scan the result the same way, then reply with the rewritten text, the hits before and after per tier, and a line for any hit left on purpose saying why. Nothing else.
-->
