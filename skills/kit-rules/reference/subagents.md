# Subagents

The Agent tool takes the namespaced name, `kit:<name>`. `/agents` lists them.

## 1. Spawn discipline

- Do the work yourself by default.
- Before a plan step that changes existing code, send the gatherer questions in `skills/investigate` to `kit:gatherer`, in one message.
- Spawn an agent only for a task matching its specialty, one needing isolation (a broad sweep, a read-only audit), or independent items that can run in parallel.
- An agent whose definition sets no `model:` runs on the session's model. Pass `model` on the call to fit the task: `haiku` for lookups and fact gathering, `sonnet` for edits that follow a given brief or template, `opus` for judgment such as review, critique, or design.
- An agent finishes its own task. A specialized agent may spawn one agent per run, only for a sub-task outside its own tools.

## 2. Verify an agent's work before building on it

- Before building on claimed edits, run `git diff --stat` on the touched paths.
- Before building on claimed findings, spot-check one cited `file:line`.
- Before synthesizing a fan-out, trace each result's decisive claim to the source.
- When two agents disagree, trace before trusting either.

## 3. What every agent definition can assume

No agent file restates these, except where noted.

- **Startup**: the resolved `<scratch>` path, repo context, and the `<required-skills>` and `<suggested-skills>` blocks arrive through the SubagentStart hooks. `researcher` ignores the repo context and `<suggested-skills>`, and loads kit-rules like any agent with Read.
- **Skills**: load every skill those blocks or the invoking skill name, via the Skill tool, before any edit or before reading the work under review. None named, proceed and say so.
- **Memory**: an agent with `memory: local` reads it before starting and records durable patterns after. Its Startup names which patterns.
- **Read-only by default**: Edit and Write exist only for memory and a scratch report. Fixes go back as instructions. `tester` and `debugger` state their write scope in their Boundaries section.
- **Output**: the invoking skill's format and path. Unbounded output (logs, traces, a full map) goes to a scratch file plus a short digest. Bounded output (a pass/fail line, a ranked shortlist) comes back inline. Writing needs `Write` in `tools:` or `memory: local`.
