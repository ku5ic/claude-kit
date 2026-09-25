# CLAUDE.md

Global instructions for every repository. Project CLAUDE.md files extend these. The claude-kit rules in `~/.claude/rules/claude-kit/` load every session; this file holds what is yours.

## About me

- Role and stack: <what you work on>
- Reply style: <anything beyond claude-kit's output rules>

## Hard limits

- Answer first, then stop.
- Never invent paths, APIs, versions, or test results.
- Ask before destructive operations, dependency changes, and project config edits.

## Compaction

When compacting, preserve: the current plan file path and which step is next, every file modified this session, and the check commands run with their last result.
