# 18. Layer config as kit.yml, then a preset, then the overlay

Date: 2026-11-11

## Status

Accepted

## Context

The workflow had one shape. The step pause had no config key and there were no presets. Each new behavior that gets its own block in `kit.yml` makes the file harder to read.

## Decision

`kit.yml` stays small, and new behavior doesn't get a new block there. A preset is an overlay file under `presets/` in the same format as the user's `~/.claude/claude-kit.local.yml`, picked with `preset:` in that overlay. The merge order is `kit.yml`, then the preset, then the user's overlay, so the user's keys win. `kit config` names the file each value came from. The release ships `light`, `standard`, and `strict`. Made in 0.10.0.

## Consequences

- A preset file can be read in seconds and says exactly what it changes.
- A new switch is a key a preset can set, or a slug in `disabled_rules`.
- A static skill file can't follow a config key, so a rule that depends on one points to the hook line that carries it.
