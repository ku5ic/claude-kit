# 3. Enforce rules with hooks, not prose

Date: 2026-09-25

## Status

Accepted

## Context

A rule in CLAUDE.md, a skill, or a prompt is advice, and Claude can skip it. Permission rules can't fill the gap. A prefix rule can't tell `git push` from `git push --force origin main`, it never sees a command inside a pipeline, and a plugin can't ship deny rules at all.

## Decision

A rule the kit means to hold runs as a hook on the command, edit, plan, subagent report, or stop it governs. A broken rule sends Claude back with a message saying what to do next.

## Consequences

- A new behavior a hook can see becomes a hook. The prose rule shrinks or goes, since the block message carries it when it matters (see [20](0020-load-the-rules-as-a-skill-behind-a-gate.md)).
- Every block is a turn the user waits through, so a new block has to earn it. `CLAUDE.md` asks for the user's view of a hook change before one ships.
- What no hook can see, such as a chat reply with no tool call, still depends on prose.
