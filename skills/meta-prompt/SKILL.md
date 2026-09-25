---
description: Turn a fuzzy ask into a sharp Claude Code prompt with context and acceptance criteria
argument-hint: <the fuzzy ask, or a file containing it>
model: haiku
disable-model-invocation: true
context: fork
---

## When to use

- You want to capture a reusable task as a new slash command
- You are about to start a new Claude Code session and want to hand off context
- You are handing a task to a teammate and want a shaped brief
- A request came in vaguely and needs sharpening before you act

## Procedure

1. Get the scratch directory: `!`scratch-dir.sh``.
2. Read $ARGUMENTS. If it points to a file, read it.
3. Run `!`detect-stack.sh`` if the ask appears technical.
4. Identify what is missing:
   - Goal unclear or conflated with method
   - Scope undefined (no files, no surface area)
   - Success unstated (how do we know it worked)
   - Constraints missing (stack, style, deadline, performance)
5. Verify before writing. For any file path, symbol, or API the prompt is about to name:
   1. Confirm it exists.
   2. Read it directly when the input names a couple of files.
   3. Delegate to the built-in Explore agent when the input references code or architecture spanning more than a couple of files.
   4. Anything that cannot be verified goes into the output as "unverified, please confirm", not asserted.
6. Rewrite as a structured prompt.

## Output file

Write to `$(scratch-dir.sh)/prompt-<slug>-<YYYYMMDD-HHMM>.md`. Print the path.

Structure:

```
# Task: <clear one-line title>

## Goal

<The outcome. What, not how.>

## Context

<Relevant files, prior work, constraints the reader needs. Link or inline.>

## Inputs

<What the agent or person has to work with. Paths, data, tickets.>

## Constraints

<Stack, style, patterns to respect, things to avoid, time bound.>

## Out of scope

<What this task does not include. Prevents drift.>

## Acceptance criteria

<List. Observable, verifiable. Not internal steps.>

## Suggested first step

<One concrete action to start. Not the whole plan.>
```

## Rules

- Extract the underlying goal, do not repeat the fuzzy phrasing.
- If the ask is actually two or three tasks bundled, split them and output multiple prompts.
- If the ask is under-specified in a way that cannot be inferred: ask via the AskUserQuestion tool (multiple-choice, "Other" for free text) before writing the output file. Record the resolved answers as a "Decisions" section instead of an unresolved "Open questions" list. If forked, follow the forked decision protocol in `rules/agents.md` instead of guessing.
- Do not name a file path, function, or API in the output unless it was confirmed this session via Read/Grep/fd or Explore; unconfirmed items are marked unverified, never asserted as fact.
- The output is meant to be copy-pasted by another agent or a human, not a chatbot.

## Bonus: promoting the prompt to a skill

If the shaped prompt looks reusable, suggest at the bottom:

> This looks reusable. Consider saving as `~/.claude/skills/<name>/SKILL.md`. Frontmatter would be:
>
> ```
> ---
> description: <one-line summary>
> argument-hint: <what fills $ARGUMENTS>
> ---
> ```
