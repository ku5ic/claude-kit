# 22. Let agents that cannot load the rules through the gate

Date: 2026-11-11

## Status

Accepted

Amends [20. Load the rules as a skill behind a gate](0020-load-the-rules-as-a-skill-behind-a-gate.md)

## Context

The gate from [20](0020-load-the-rules-as-a-skill-behind-a-gate.md) denied every tool call of the built-in web-fetch agent. That agent has neither the Skill tool nor Read, so it could never load the rules, and every web fetch failed while the kit was on. The hook payload doesn't say which tools an agent has.

Two simpler fixes were rejected. Denying once per agent lets any agent past the gate by calling again. A list of exempt agent types in `kit.yml` has to be kept up by hand, and it misses every other plugin's agents.

## Decision

The gate reads the tools Claude Code gave the agent from the agent's own transcript, `<session>/subagents/agent-<agent_id>.jsonl`. Its `api-request-shape` record is written before the agent's first request, and its `tools` lists the tools offered (`transcript.ReadAgentStart`).

- An agent given neither Skill nor Read passes, and the skip is logged as `rules-gate-unloadable`.
- An agent given either is gated as before.
- A fork has no shape record. It starts from a `fork-context-ref` record with its parent's context, so its parent's load counts for it.
- An agent whose record can't be read is gated, as before.

## Consequences

- No agent gets past the gate by how it behaves. The exemption follows from what Claude Code gave the agent.
- An agent with no way to load the rules runs without them. Its tools limit what it can do.
- Both records are undocumented. On 2026-11-11, 39 of the last 40 subagent transcripts held a shape record, and the 40th was a fork. If Claude Code drops the record, tool-limited agents are denied again, as before this ADR, and `rules-gate-unloadable` stops appearing in the guards log.
