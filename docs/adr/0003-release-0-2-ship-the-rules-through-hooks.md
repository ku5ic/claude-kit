# 3. Release 0.2: ship the rules through hooks

Date: 2026-10-06

## Status

Superceded by [9. Release 0.10: the engineer in the loop, the rules as a gated skill, a visible claim checker](0009-release-0-10-the-engineer-in-the-loop-the-rules-as-a-gated-skill-a-visible-claim-checker.md)

## Context

This record replaces the former ADR 7, and its sources are that record's. Up to 0.2.0, the rules reached a session through `install-rules.sh`, which linked or copied the rule files into `~/.claude/rules`, a separate step from installing the plugin.

## Decision

From 0.2.1 (PR #5), the `inject-rules` hook adds `rules/*.md` to every session at SessionStart and to every subagent at SubagentStart. A leftover `~/.claude/rules` link or copy is reported, because the rules would otherwise load twice.

## Consequences

- The rules reached every session and subagent with no install step.
- They loaded out of sight, about 43 KB at every start, and the user never saw them load.
- A subagent paid for every rule file, whether it needed it or not.
