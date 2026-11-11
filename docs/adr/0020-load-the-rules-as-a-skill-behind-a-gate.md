# 20. Load the rules as a skill behind a gate

Date: 2026-11-11

## Status

Accepted

Supercedes [7. Ship the rules through hooks](0007-ship-the-rules-through-hooks.md)

## Context

Injected rules loaded out of sight, 42,919 bytes at every session and subagent start ([7](0007-ship-the-rules-through-hooks.md)). The chat bundle already built a `kit-rules` skill ([13](0013-generate-the-chat-bundle-from-the-same-files.md)), so the name and shape had precedent.

## Decision

The rules are the `kit-rules` skill, in `skills/kit-rules/`. A catch-all PreToolUse hook denies any tool call until the skill is loaded, for the main session and for each subagent on its own load. An agent without the Skill tool loads it by Read. The deny message is an order to load the skill and retry, because the first live test showed Claude stopping to ask instead. Hook injection is removed. Each rule is one line, a condition then an action, and its reasoning lives in `docs/rules.md`, which the model never loads. Made in 0.10.0.

## Consequences

- The load is visible as a tool call, and the rules cost about 28 KB, down from 43 KB.
- A reply that makes no tool call can't be gated. The session-start instruction covers it, and the release eval measures it.
- A rule a hook enforces leaves the text ([3](0003-enforce-rules-with-hooks-not-prose.md)).
- `disabled_rules: [rules-gate]` turns the gate off.
