---
name: code-tour
description: An onboarding walk through a repo: where it starts, one path end to end, where it changes, who to ask
argument-hint: "[repo path or subdirectory] [focus: a request, command, or feature to trace]"
disable-model-invocation: true
allowed-tools:
  - Bash(gh pr diff *)
  - Bash(gh pr view *)
  - Bash(glab mr diff *)
  - Bash(glab mr view *)
  - Bash(git log *)
  - Bash(kit code-graph *)
  - Bash(kit git-base *)
  - Bash(kit scratch-dir *)
---

# /code-tour

An onboarding walk through a repo for someone new to it: where it starts, one path traced end to end, where it changes most, where tests and config live, and who to ask.

## Inputs

1. Root: `git -C <path or cwd> rev-parse --show-toplevel`. A subdirectory argument narrows every step to it.
2. Read the project CLAUDE.md, README, and CONTRIBUTING first. The tour uses the project's own names, and links to what those files already explain instead of restating it.
3. When `/code-visual` has run, its `<arch>/README.md` (`skills/code-visual/SKILL.md` Inputs step 5) holds the containers and module graph. First run `kit code-graph check-c4 <arch> <raw> <root>`, with `<raw>` from `visual.md` Collect step 4, and its fallback without ast-grep. It passes: link the README, and skip drawing them again. It lists problems: name them, and ask with AskUserQuestion whether to refresh the model with `/code-visual` first, or go on without it.

## Collect

1. **Entry points.** Whatever starts the code: the entry and script fields of each manifest, the files that start a process or a CLI, route definitions, container commands, and CI's deploy job. Confirm each by reading it.
2. **One path, end to end.** The focus argument, or the most common entry: one request through its handler to storage and back, or one command from argument parsing to output. Read every file on the way and note `file:line` at each hop.
3. **Hot spots.** `skills/code-visual/SKILL.md` Collect steps 1 to 3, with a window of `6.months`. The top 5 only.
4. **Tests and config.** Where tests live and how they're run (the project's own script or task), the fixtures folder, env files and their example, and the config each tool reads.
5. **Who to ask.** `skills/code-history/SKILL.md` Digest step 5, run on each top-level module.

## Report shape

Write to the path `kit scratch-dir tour <repo-slug>` prints.

```markdown
# <project>: tour

Root: <root> at <short HEAD hash>. Architecture: <link to `<arch>/README.md`, or "run `/code-visual`">.

## TL;DR

- <what it does>
- <the one folder to read first>
- <the command that runs it locally>

---

## Entry points

| Entry | Kind | File |
| ----- | ---- | ---- |

---

## <The traced path, named>

<One numbered step per hop, each one line with file:line and what happens there.>

---

## Where it changes

| File | Commits | Complexity | Top owner |
| ---- | ------- | ---------- | --------- |

- **<file>**: <why it keeps changing, from its commit subjects>

---

## Tests and config

- <where tests live, the command that runs them, fixtures>
- <config files and what reads each>

---

## Who to ask

- **<module>**: <author (commits)>
```

Readability follows the report-format skill's Readability section. A why that comes from neither code nor a commit is labelled per `skills/kit-rules/reference/evidence.md` section 1.

## Reply

The report path, the first TL;DR bullet, and one next action.

## Out of scope

- Judging the code: that's `/code-deep-review` or an `audit-*` skill.
- Diagrams beyond what `<arch>` already holds: that's `/code-visual`.

Arguments: `$ARGUMENTS`
