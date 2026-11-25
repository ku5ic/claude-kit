---
name: audit-slop
description: Survey a repo's prose for AI tells and filler, worst files first
argument-hint: "<file, directory, or none for the whole repo>"
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
---

# /audit-slop

Read-only survey of a repo's prose: where the AI tells and the filler sit, ranked so the worst files come first. Rewriting is `/unslop`'s job.

## Run

Resolve a linked ticket or doc as `skills/investigate` step 0 describes, then work this file's procedure on the resolved arguments per `skills/kit-rules/reference/subagents.md` section 4, each part in a `kit:auditor` or one auditor for all of it, and write the report yourself. Then run the report-format skill's Runtime checks yourself, and reply per `skills/kit-rules/reference/output.md` section 3.

## Procedure

1. Load the slop-patterns skill and read its `principles.md` and `content-and-grammar.md`.
2. A file target is the one file to scan. Otherwise list the prose files in the directory or repo: `fd -e md -e mdx -e markdown -e txt . <target>`. Skip generated files and changelogs.
3. Scan them: `kit slop <files>`. Each output line is `path:line: T<tier> rule "text" -> fix`.
4. Rank the files by tier 1 and 2 hits, with `rg ': T[12] ' | cut -d: -f1 | sort | uniq -c | sort -rn` on the scan output.
5. Read the top 5 files, and every file whose hits are mostly tier 3 density, for what no scan sees:
   - a sentence that carries no fact, per `principles.md`
   - a section that restates another file. When the project's CLAUDE.md names an owner file per topic, a restatement outside its owner is a `warning`.
   - every row `content-and-grammar.md` marks "none, judgment"
6. A file the scan flags but that is a quotation, an example of a tell, or a lexicon is a false positive (slop-patterns `false-positives.md`). It goes under "Out of scope", not findings.

## Output per finding

- The file, its hit count by tier, and the worst lines with `file:line`
- Evidence: the scan line, or the quoted sentence and the fact it lacks
- Severity: tier 1 hits are `failure`, tier 2 and a restated owner `warning`, density and judgment calls `info`
- Fix: `/unslop <file>`, or the file the restatement should link to instead

## Output file

Load the report-format skill and use its format. Write to the path `kit scratch-dir slop <target-slug>` prints.

Arguments: `$ARGUMENTS`
