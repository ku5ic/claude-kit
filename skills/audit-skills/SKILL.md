---
name: audit-skills
description: How skills, commands, and agents are written, against Claude Code's own prompt audit and Anthropic's current guidance
argument-hint: "<skill or agent names, paths, or all>"
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
  - Bash(kit scratch-dir *)
---

# /audit-skills

Read-only audit of how skills, commands, and agents are written, used or not. Whether a skill's content is still true to its sources is `/meta-refresh`'s job, and what sessions show is `/meta-improve`'s.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before             | Read                                                                                                |
| ------------------ | --------------------------------------------------------------------------------------------------- |
| Judging any target | `skills/meta-skill/reference/skill-authoring.md`, `skills/meta-prompt/reference/prompt-practice.md` |

## Run

1. Targets, from the arguments: names, paths, or `all`. None, ask. They're the directories with a `SKILL.md` and the agent and command files of the plugin (`skills/`, `agents/`) when the project root has `.claude-plugin/plugin.json`, else of `.claude/skills/`, `.claude/agents/`, and `.claude/commands/`. A skill's target is its whole directory. When a name matches nothing, say so and ask. More than 5: list them and ask before going on.
2. Dispatch the researcher agent (subagent_type: kit:researcher, foreground). Ask it to fetch, from Anthropic's own documentation only, the pages on writing skills, on subagents, and on prompting each model the targets run on: the one each pins, else the session's. Ask for each rule a reviewer could check in a file, quoted, with its URL, and for each rule in the two files above whether a page confirms, contradicts, or doesn't mention it. Write its reply to `practice.md` in the report's raw folder.
3. Dispatch the auditor agent (subagent_type: kit:auditor, foreground), one per 5 targets, with the Procedure below, the targets, and the absolute path of `practice.md`.
4. Reply per `skills/kit-rules/reference/output.md` section 3.

## Procedure

1. Run the built-in audit. Load the `claude-api` skill through the Skill tool with `prompt-audit` and the targets' paths as its arguments, and follow its guide on those paths. It asks for a report and a diff. Its findings go into this report, and its diff becomes each one's Fix. Nothing is applied. When that skill isn't there, stop and return this line for the user to type, one per target, so its report can be read: `/doctor prompt-audit <path>`.
2. Check every path, skill, agent, command, and section a target names. One that doesn't exist is a `failure`.
3. Check each target against its own files: two instructions that can't both be followed, a thing its description promises and its body drops, a step no tool in its frontmatter can do.
4. Check each against the rules in `practice.md` and the two files in the table. Name which one each finding rests on, by URL or `file:line`.
5. A page that contradicts one of those two files is one finding against that file, its Fix `/meta-refresh <skill>`, never a finding against each target.

## Not a finding

- What the built-in guide's keep list keeps.
- A rule the project's own instructions set on purpose: a byte budget, a required table, a fixed phrase a test checks. Read the project's CLAUDE.md and its tests for the file first.
- How well a skill works. Nothing here measures it, so the report has no score and no ranking.
- Length alone.

## Output per finding

- The target and its `file:line`, with the text quoted
- Evidence: the second side of the contradiction, the missing path, the guide's pattern, or the fetched rule with its URL
- Severity: `failure` for a contradiction or a path that doesn't exist, `warning` for text that steers a current model wrong, `info` for what the built-in guide would only flag
- Fix: the replacement text, noted and never applied

## Output file

Load the report-format skill and use its format. Write to the path `kit scratch-dir skills <target-slug>` prints. Its Scope line names every target, its model, and the date of `practice.md`. The report is what `/plan` reads next.

Arguments: `$ARGUMENTS`
