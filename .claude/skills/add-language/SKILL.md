---
name: add-language
description: Add a programming language to the kit, its Go support and its pattern skill, from primary sources
argument-hint: "<language, e.g. kotlin or elixir>"
disable-model-invocation: true
---

# /add-language

Teach the kit a language: detect it, read its imports, scan its comments, guard its tests and packages, and load a pattern skill for it. A framework on a language the kit knows needs `/meta-skill` alone.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                     | Read                                       |
| -------------------------- | ------------------------------------------ |
| Researching, then planning | [reference/owners.md](reference/owners.md) |

## Procedure

1. Scope. The language is: $ARGUMENTS. Blank: ask via AskUserQuestion. Work in the kit's source checkout, as `skills/meta-skill/SKILL.md` step 4 defines it; anywhere else, stop and say so. Find what the kit already has for it, with `rg -n -i '<language>|<its extensions>'` over `go/internal`, `kit.yml`, and `kit.d/`, and keep only the rows of the owners table it lacks.
2. Research. Dispatch the researcher agent (subagent_type: kit:researcher, foreground) with the questions in [reference/owners.md](reference/owners.md) "What to ask", for this language. One primary source per answer (the language spec, the toolchain's docs, the registry's docs, ast-grep's language list). An answer without a source leaves its row out of the plan, named as skipped.
3. Plan. Enter plan mode and write one step per owners row kept, in the table's order, each naming its owner, its test, and the probe that shows it working. The last two steps run `skills/meta-skill/SKILL.md`'s procedure for the pattern pack and add the docs rows. Stop for approval.
4. Each step: the test first, run red, then the change, run green, then the step's probe with the built binary (`KIT_DEV=1 bin/kit ...`). Stop after each step for review, per `skills/kit-rules/reference/workflow.md`.
5. Done when `go -C go test -race ./...` and the gate pass, and a fixture repo in the language shows it in `kit detect-stack` and its imports in `kit blast-radius`.

## Rules

- Nothing from memory. Manifest names, import forms, comment syntax, and registry commands come from step 2's sources.
- Change each fact in its owner only. A second list of the language's extensions, import forms, or test-file names anywhere else is a defect.
- A row the language has nothing for (no package registry, no workspaces) is skipped and named in the plan, never filled with a guess.
