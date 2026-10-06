# claude-kit

Working on the kit itself. Build, test, and release steps live in README.md "Develop"; this file holds only what that doesn't.

## One source of truth

Every behavior the kit applies everywhere has exactly one owner, listed below. Before adding or changing guidance, find its owner here: change it there, or nowhere. Every other file links to the owner or says nothing about the topic. It never restates, paraphrases, or reminds.

- Data and mechanics belong to code: `kit.yml`, the `kit` binary, `bin/*.sh`. Prose never re-describes what a script does.
- When two files disagree, the owner wins and the other file is fixed.
- A topic missing from this table gets a row before it gets text anywhere else.

| Topic                                                                                               | Owner                                                    |
| --------------------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| Reply length, shape, where output goes, writing files with Edit/Write, code-block tags              | `rules/output.md`                                        |
| Evidence, confidence labels, never invent, stop after three failed fixes                            | `rules/evidence.md`                                      |
| Minimal change, precedent, blast radius, dead code, comments, abstractions, simplification criteria | `rules/change.md`                                        |
| Done and verification                                                                               | `rules/verify.md`                                        |
| Git and commit policy, side effects, ask vs proceed, AskUserQuestion                                | `rules/workflow.md`                                      |
| Subagent use and agent defaults                                                                     | `rules/subagents.md`                                     |
| CLI choice, calling bin scripts, scratch location                                                   | `rules/tooling.md`                                       |
| Artifact naming and path                                                                            | `bin/scratch-dir.sh`                                     |
| Resolving a linked ticket or doc before work starts                                                 | `skills/investigate` step 0                              |
| Report shape and severity                                                                           | `skills/report-format`                                   |
| Test design                                                                                         | `skills/test-patterns`                                   |
| Stack detection, guards, checks, formatters, tools                                                  | `kit.yml`                                                |
| Personal register and typography                                                                    | the user's own `~/.claude/rules/voice.md`, never the kit |

Skills and agents carry procedure and stack knowledge only. When one needs a rule above, it cites `rules/<file>.md` instead of restating it.

## Rules are live

The SessionStart and SubagentStart hooks inject `rules/*.md` from the plugin root they run from. A session running the hooks from this checkout picks up a rule edit at its next start or compaction, so finish a rules change in one sitting.
