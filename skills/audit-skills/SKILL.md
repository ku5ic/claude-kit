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
2. Read each target's last `skills-*` report in `kit scratch-dir`, when there is one. A target unchanged since it is skipped, by `git log` and `git status`. One changed only to fix its findings gets the line `/audit-verify <report>` in the reply and no new audit.
3. Start the practice and the angles below together. The practice: reuse a `practice.md` in a `skills-*` raw folder fetched today that covers every target's model. Otherwise dispatch the researcher agent (subagent_type: kit:researcher). Ask it to fetch, from Anthropic's own documentation only, the pages on writing skills, on subagents, and on prompting each model the targets run on: the one each pins, else the session's. Ask for each rule a reviewer could check in a file, quoted, with its URL, and for each rule in the two files above whether a page confirms, contradicts, or doesn't mention it. Write its reply to `practice.md` in the report's raw folder, with the date.
4. Fan out per `skills/kit-rules/reference/subagents.md` section 4, one auditor per angle and per 30 targets, the targets split so each batch holds about the same amount of text. Guidance starts once `practice.md` is written, and the researcher counts as one of the running agents. Each auditor gets The bar, Not a finding, Output per finding, its targets, and the last reports' paths, and Guidance the absolute path of `practice.md`. It writes `candidates-<angle>-<n>.md` and also returns the targets it didn't finish.
5. Merge with the stronger Source kept, not the higher severity. Then refute. Dispatch one agent per target with findings (general-purpose, model `sonnet`), under the same limit on running agents, with that target's findings, its path, and The bar, told to show each one wrong. For each it answers each line of The bar with what it read, then `stands` or `broken`. A finding it can't settle, or doesn't reach, is `broken`.
6. Read the decisive line of each finding that stands yourself (`skills/kit-rules/reference/subagents.md` section 2). One you can't confirm is broken too. Write the report from what's left, and the broken ones, each with what broke it, to `rejected.md` in the raw folder.
7. Reply per `skills/kit-rules/reference/output.md` section 3.

## Angles

Each auditor takes one, over every file of its targets, and reports nothing that belongs to another.

### Prompt

Call the Skill tool with `skill` set to `claude-api` and `args` set to `prompt-audit` and the targets' paths, and follow its guide on those paths. It asks for a report and a diff. Its findings are candidates for The bar, and its diff becomes each one's Fix. Nothing is applied. When that skill isn't there, stop and return this line for the user to type, one per target, so its report can be read: `/doctor prompt-audit <path>`.

### Names

Every path, section, skill, agent, command, subcommand, flag, and tool a target names exists and does what the target says it does.

### Contradictions

Two instructions that can't both be followed: inside a target, or between a target and a project rule it cites or falls under. A step no tool in its frontmatter can do.

### Gaps

What a target leaves out, shown by its own text:

- A thing its description or a heading promises, and no line delivers.
- An input its arguments, steps, or tables admit (none, several, one that isn't there, a failed step), with no instruction for it.
- A step that needs a fact, a file, or a tool no earlier step provides.
- A reference file no row links, and a row whose file doesn't hold what the row says.

### Guidance

Each rule in `practice.md` and the two files in the table. A page that contradicts one of those two files is one finding against that file, its Fix `/meta-refresh <skill>`, never a finding against each target.

## The bar

A finding meets every line, or it's dropped:

- **Quote.** The target's text at its `file:line`, read in this run, word for word.
- **Source.** The second instruction, the path that doesn't exist, the line that promises or needs what's missing, or the rule it breaks, quoted with its `file:line` or URL. A rule that says "can", "consider", or "usually" breaks nothing.
- **Case.** One concrete input, and the wrong result that follows from the text as written. "Could be read as", "might", and a reader who ignores the rest of the file are no case.
- **Fix.** The replacement text. With it applied, the case gives the right result, and nothing else in the target, the two files in the table, or the project's rules contradicts it. A second place that has to change with it is in the Fix.
- **New.** Not a finding the last report fixed or rejected, nor the reverse of its Fix, unless the source changed since, quoted, or the Case shows that Fix itself giving a wrong result.

## Not a finding

- What the built-in guide's keep list keeps, and what it would only flag.
- A rule the project's own instructions set on purpose: a byte budget, a required table, a fixed phrase a test checks. Read the project's CLAUDE.md and its tests for the file first.
- How well a skill works. Nothing here measures it, so the report has no score and no ranking.
- Length, word choice, and order alone.

## Output per finding

- The target and its `file:line`, with the text quoted
- Evidence: the source, then the case and its wrong result
- Severity: `failure` when the text can't be followed (a contradiction, or a path that doesn't exist), `warning` for any other finding. There is no `info`
- Fix: the replacement text, noted and never applied

## Output file

Load the report-format skill and use its format. Write to the path `kit scratch-dir skills <target-slug>` prints. Its Scope line names every target, its model, the date of `practice.md`, how many findings were refuted, and each target no auditor finished. No finding standing: the report says so in one line. It's what `/plan` reads next.

Arguments: `$ARGUMENTS`
