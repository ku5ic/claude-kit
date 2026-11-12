# 17. Guard only the kit's own off switches

Date: 2026-11-11

## Status

Accepted

## Context

A 0.9.0 audit showed the kit could be switched off quietly. `KIT_VERIFIER=1` in the environment made every hook return 0, and a write to `.claude/settings.local.json` or `~/.zshenv` passed both guards. The same audit's probes found other commands the guards let through.

## Decision

The guards protect the kit's own off switches. Writes to Claude Code settings files, the installed plugin root, `~/.claude/logs`, and `~/.claude/cache` ask, and `~/.zshenv` is treated like `~/.zshrc`. A guard variable is honored only with a marker named by its value's hash, with a depth counter against re-spawning. Everything else the audit's probes passed goes in a residual-risk list. Decided by the author for 0.10.0.

## Consequences

- Turning the kit off takes a step the user sees.
- The guards aren't a sandbox. The residual-risk list says what they don't stop.
- A request for wider bash hardening is a new decision, not a guard tweak.
