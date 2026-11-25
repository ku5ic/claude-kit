# Subagents

The Agent tool takes the namespaced name, `kit:<name>`. `/agents` lists them.

## 1. Spawn discipline

- Do the work yourself by default.
- When a plan step sends `kit:gatherer` its questions: `skills/investigate`, Gatherer questions.
- Spawn an agent only for a task matching its specialty, one needing isolation (a broad sweep, a read-only audit), or independent items that can run in parallel.
- An agent whose definition sets no `model:` runs on the session's model. Pass `model` and `effort` on the call, by section 4's table.
- An agent finishes its own task. A judging agent, such as plan-critic, may send its fact questions to read-only `kit:gatherer` runs in one parallel message, then judge from their reports. Any other specialized agent may spawn one agent per run, only for a sub-task outside its own tools.
- Claude Code allows three layers of subagents below the main conversation (`CLAUDE_CODE_MAX_SUBAGENT_SPAWN_DEPTH`) and 20 running in a session (`CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS`), and a spawner needs `Agent` in its `tools` (code.claude.com/docs/en/sub-agents).

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

## 4. Fan-out

A skill with an `## Angles` table starts one agent per row, never one agent for all of them. Where the skill's own steps say otherwise, they win.

- First collect what more than one angle needs, once, in the main session: the resolved target, anything read through a connector, a graph, a token list, the routes. It goes in the report's raw folder.
- A row with nothing in the target to read is skipped, and the report says so. A single-file target still starts one agent per remaining row.
- Agents go in one message, no more at once than `CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS` allows (20 when unset), the rest as those finish.
- Each is a `kit:auditor` run unless the skill names another agent. It gets the SKILL.md path, its row, the target, and the raw folder, and does only its row. Of the suggested skills it loads only what its row's Reads names, the exception to section 3. It writes `candidates-<angle>.md` in the raw folder and returns the path. In plan mode it returns its findings inline.
- With every agent back, merge findings on the same line or with the same cause, keep the higher severity, and carry over each "Needs a runtime check". Trace each kept finding's decisive claim (section 2), since the claims check reads this session's transcript, then write the report. It names an angle whose agent failed.
- The row's Task is its usual one. The main session may take another row of this table for a harder or easier target, and the report says why.

| Task    | What the agent does                                                    | Model    | Effort   |
| ------- | ---------------------------------------------------------------------- | -------- | -------- |
| Collect | runs a tool, counts, or lists facts by path                            | `haiku`  | unset    |
| Check   | checks code against a stated list, reviews a diff, or edits to a brief | `sonnet` | `medium` |
| Judge   | weighs a design, a plan, a decision, or a tradeoff                     | `opus`   | `high`   |
