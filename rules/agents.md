# Agents

Subagent shells live under `$HOME/.claude/agents/`. The canonical inventory is `/agents` output.

## Spawn discipline

A subagent costs its own request budget, so default to doing the work directly. Spawn one only when the task matches an agent's specialty, needs isolation from the main context (a broad sweep, a read-only audit), or genuinely parallelizes across independent items. Spawn `Explore` only past 5 unresolved queries, not 3.

An agent finishes its own task rather than spawning further subagents. Nested spawning is allowed only from a specialized agent, for a sub-task outside its own tool grant, at most one per invocation.

## Model and effort pins

Pin `model:` or `effort:` frontmatter only when it diverges from the session default; `doctor.sh` flags a redundant pin. A skill may set `model:` or `effort:`, never both (Claude Code drops the model override when both are present on a skill). A skill forks (`context: fork`) only to carry a `model:` pin or to name an `agent:`.

## Forked decision protocol

1. A forked skill has no AskUserQuestion tool.
2. A step that would ask instead stops and returns the question(s) and options under a `## Needs decision` heading.
3. On a task-notification carrying that heading, ask via AskUserQuestion in the main conversation, then resume the same agent via SendMessage with the answer(s).

## Verify agent-claimed work before building on it

An agent's report describes what it intended, not necessarily what it did. Before committing, reporting, or handing a claim to another agent: claimed edits get a `git diff --stat` on the touched paths; claimed findings get one cited `file:line` spot-checked.

Before synthesizing a fan-out, each agent's findings get one more pass: identify the single claim that result's viability rests on and trace it to the source yourself. Two agents disagreeing means neither is correct until traced. A couple of tool calls, not a re-run of the agent's work.

## Agent shell boilerplate

Every agent definition assumes these three, so no agent file restates them except where noted:

- **Startup**: the resolved `<scratch>` path, repo context, and the `<required-skills>`/`<suggested-skills>` blocks arrive via the `SubagentStart` hook (`inject-subagent-context.sh`), independent of the agent's tool grants. Agents that need no repo context (`checker`, `researcher`) ignore the block.
- **Read-only boundary**: the default. Edit and Write exist only for memory and a scratch report; never touch project source, state fixes as instructions. `tester` and `debugger` hold scoped write grants instead - test files and a single reverted probe edit - and each states its own scope in its Boundaries section.
- **Output**: follow the invoking skill's format and path. Unbounded output - logs, traces, a full map - goes to a named scratch path plus a short digest. Output the agent's own contract already bounds, like a pass/fail line or a ranked shortlist, is returned inline and never written to a file. Writing needs a real grant: `Write` in `tools:`, or the one `memory: local` implies. An agent without that grant returns everything in its response.
- **How a file gets written**: every artifact - scratch report, memory file, deliverable - is written with Write or Edit, at an absolute path. Never `cd` into the destination and redirect to a bare filename: `guard-bash.sh` reads a bare redirect target as a write into the repo root and forces a confirmation prompt, which is a false positive inside a memory or scratch directory. A shell redirect is for capturing command output, and then the target is `"$(scratch-dir.sh)/<name>"`, never a bare name.
