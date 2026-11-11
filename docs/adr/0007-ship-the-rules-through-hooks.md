# 7. Ship the rules through hooks

Date: 2026-11-11

## Status

Superceded by [20. Load the rules as a skill behind a gate](0020-load-the-rules-as-a-skill-behind-a-gate.md)

## Context

Up to 0.2.0, the rules reached a session through `install-rules.sh`, which linked or copied the rule files into `~/.claude/rules`, a separate step from installing the plugin.

## Decision

From 0.2.1 (PR #5), the `inject-rules` hook adds `rules/*.md` to every session at SessionStart and to every subagent at SubagentStart. A leftover `~/.claude/rules` link or copy is reported, because the rules would otherwise load twice.

## Consequences

- The rules reached every session and subagent with no install step.
- They loaded out of sight, about 43 KB at every start, and the user never saw them load.
- A subagent paid for every rule file, whether it needed it or not.
