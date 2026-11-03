---
name: unslop
description: Find and rewrite AI tells and filler in a file, a diff, or pasted text, then show what changed
argument-hint: <file...|diff|-> [in place]
disable-model-invocation: true
allowed-tools:
  - Bash(kit slop *)
  - Bash(kit scratch-dir *)
  - Bash(git diff *)
---

## Procedure

1. Load `slop-patterns`. It says how to rewrite; this skill only says what to run on.
2. Resolve the target from the arguments:
   - Files: each path.
   - `diff`: the files `git diff --name-only` lists, and only their changed lines count.
   - `-` or no argument: ask for the text, then treat it as one file.
3. Scan: `kit slop <files>`. For pasted text, `kit slop --surface <file|commit|pr|chat> -` with the text on stdin; ask which surface when it isn't obvious.
4. Rewrite every tier 1 and 2 hit, and every passage the scan reports as dense, per `slop-patterns` `reference/rewrite.md`. Then read the whole text once for what no regexp sees: sentences that carry nothing, sections that restate others (`reference/principles.md`). For UI files, also walk `reference/design.md` section 12.
5. Write the result:
   - Default: a copy at the path `kit scratch-dir unslop <slug>` prints, one per file.
   - `in place` in the arguments: edit the file itself.
6. Re-scan the result with `kit slop`. A hit left on purpose gets a line in the reply saying why.

## Output

Per `rules/output.md` section 3: the path of each result, hits before and after per tier, and `git diff --no-index <original> <result>` for a copy. Nothing else.

Arguments: `$ARGUMENTS`
