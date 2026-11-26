# Subagents

The Agent tool takes the namespaced name, `kit:<name>`. `/agents` lists them.

## 1. Spawn discipline

- Do the work yourself by default. A skill's independent parts follow section 4.
- When a plan step sends `kit:gatherer` its questions: `skills/investigate`, Gatherer questions.
- Spawn an agent only for a task matching its specialty, one needing isolation (a broad sweep, a read-only audit), or independent items that can run in parallel.
- An agent whose definition sets no `model:` runs on the session's model. For such an agent, pass `model` and `effort` on the call, by section 4's table.
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
- **Skills**: load every skill those blocks or the invoking skill name, via the Skill tool, or without it a Read of its SKILL.md, before any edit or before reading the work under review. None named, proceed and say so.
- **Memory**: an agent with `memory: local` reads it before starting and records durable patterns after. Its Startup names which patterns.
- **Read-only by default**: Edit and Write exist only for memory and a scratch report. Fixes go back as instructions. `tester` and `debugger` state their write scope in their Boundaries section.
- **Output**: the invoking skill's format and path. Unbounded output (logs, traces, a full map) goes to a scratch file plus a short digest. Bounded output (a pass/fail line, a ranked shortlist) comes back inline. Writing needs `Write` in `tools:` or `memory: local`.

## 4. Fan-out

When a skill's work has independent parts, the main session splits it and starts one agent per part. A part is an angle the skill's text names over one target (a category, a pass, a check), or an item (a file, a finding, a candidate, a comment). The main session decides at run time, and says how it split the work, or why it didn't, in the report's Scope line, or in the reply when the skill writes none.

- Split when two or more parts need nothing from each other. Each angle gets its own agent; items may go in batches of about equal size. Keep in the main session a text being drafted, a step that needs the last one's result, and anything that posts, merges, or changes files. Work with one part runs the way the skill already runs it.
- First collect what more than one part needs: the resolved target, anything read through a connector, a graph, a list, a scan. It goes in the report's raw folder, else scratch.
- Agents go in one message, no more at once than `CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS` allows (20 when unset), the rest as those finish.
- Each is the read-only agent whose definition fits, `kit:auditor` unless another does. It gets the SKILL.md path, its part, the target, and the raw folder. It does only its part, never the report, and loads only the references its part needs, the exception to section 3's suggested skills. It writes `candidates-<part>.md` in the raw folder and returns the path. In plan mode, or with no Write tool, it returns them inline and the main session saves them.
- With every agent back, merge findings on the same line or with the same cause, keep the higher severity, carry over each "Needs a runtime check", trace each kept finding's decisive claim (section 2), then write the report. It names a part whose agent failed.
- The part's task picks its model and effort from the table, passed on the call, unless the agent's definition sets its own `model:`. Another row fits a harder or easier part, and the report says why.
- A skill's own steps win where they differ, but an order it gives its parts ranks the report and doesn't stop the split.

| Task    | What the agent does                                                    | Model    | Effort   |
| ------- | ---------------------------------------------------------------------- | -------- | -------- |
| Collect | runs a tool, counts, or lists facts by path                            | `haiku`  | unset    |
| Check   | checks code against a stated list, reviews a diff, or edits to a brief | `sonnet` | `medium` |
| Judge   | weighs a design, a plan, a decision, or a tradeoff                     | `opus`   | `high`   |
