# 2. Use Claude Code built-ins over custom copies

Date: 2026-11-11

## Status

Accepted

## Context

Before the kit was a plugin (commit 909e23e), the setup had grown its own copies of what Claude Code ships: plan mode, code review, security review, Explore, and resume. Eleven `flow-*` skills did this work. The setup cost about 47.5k tokens before any work started, and it could only be used by cloning a dotfiles repo.

## Decision

Where Claude Code has a built-in for a job, the kit uses it and adds hooks around it. It doesn't ship its own version. The workflow runs on `/plan`, `/code-review`, `/verify`, and Explore, and the kit's own skills cover only what no built-in does.

## Consequences

- Before adding a skill or an agent, check whether a built-in already does the job.
- Per-session context stays small, because built-ins cost the kit nothing to carry.
- When a built-in changes its behavior, the kit's hooks around it may need to follow.
