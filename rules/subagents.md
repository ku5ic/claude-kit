# Subagents

Agent definitions live in the plugin's `agents/`; `/agents` lists them. The Agent tool takes the namespaced name, `kit:<name>`; a bare `auditor` fails as not found.

## 1. Spawn discipline

A subagent costs its own request budget, so default to doing the work directly. Spawn one only when the task matches an agent's specialty, needs isolation from the main context (a broad sweep, a read-only audit), or splits into independent items that can run in parallel.

An agent finishes its own task rather than spawning further subagents. Nested spawning is allowed only from a specialized agent, for a sub-task outside its own tool grant, at most one per invocation.

## 2. Verify an agent's work before building on it

An agent's report describes what it intended, not necessarily what it did. Before committing, reporting, or handing a claim to another agent: claimed edits get a `git diff --stat` on the touched paths; claimed findings get one cited `file:line` spot-checked.

Before synthesizing a fan-out, each agent's findings get one more pass: identify the single claim that result's viability rests on and trace it to the source yourself. Two agents disagreeing means neither is correct until traced. A couple of tool calls, not a re-run of the agent's work.

## 3. What every agent definition can assume

No agent file restates these, except where noted:

- **Startup**: the rules, the resolved `<scratch>` path, repo context, and the `<required-skills>`/`<suggested-skills>` blocks arrive through the SubagentStart hooks, whatever the agent's tools. `checker` and `researcher` ignore the repo context.
- **Read-only by default**: Edit and Write exist only for memory and a scratch report. Never touch project source; state fixes as instructions. `tester` and `debugger` hold scoped write grants instead, and each states its scope in its Boundaries section.
- **Output**: follow the invoking skill's format and path. Unbounded output (logs, traces, a full map) goes to a scratch file plus a short digest. Output the agent's contract already bounds, like a pass/fail line or a ranked shortlist, comes back inline. Writing needs a real grant: `Write` in `tools:`, or the one `memory: local` implies.
