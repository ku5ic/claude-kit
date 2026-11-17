# 12. Release 0.12.1: mods for what the kit draws

Date: 2026-11-17

## Status

Accepted

## Context

Sources are the commits on `feat/subagent-rows-as-a-mod`, each named where it's used.

- Claude Code 2.1.326 warns that `subagentStatusLine` "is deprecated in favor of mods and will stop working in a future release", and `kit subagent-statusline` fed that setting.
- A mod is a TypeScript hooks module that `hooks/hooks.json` names under `modules`. It imports only the plugin's own files and runs with no Node and no npm, so it can't be part of the Go binary ADR 2 describes.

## Decision

- What the kit draws in Claude Code's interface is a mod in `mods/`, loaded through one entry, `mods/index.ts`, with a folder per mod (`c3f6566`). This reverses ADR 2's "Every hook and helper is one Go binary" for mods only. Command hooks and helpers stay in the binary.
- Each mod has its own `userConfig` switch. The first, subagent rows, is on by default: name, status, model and effort, context used, time running, and the tool the agent is running (`c3f6566`).
- `kit subagent-statusline` is gone, and the status line itself stays a command.
- No `package.json`: `claude plugin validate` type-checks `mods/`, and `claude plugin test` runs its tests on a copy of the plugin. pnpm comes in when a mod needs a dev tool or a bundled library.
- A change to `mods/` bumps the version, as a change to `go/` does (`c3f6566`).

## Consequences

- A mod depends on an early-access API that "moves between releases", so CI pins the Claude Code version it validates and tests against.
- Claude Code 2.1.323 accepts the `modules` key and has no agent rows for a plugin to draw, so it shows its own (123333e). A process with mods switched off by Claude Code's rollout switch does the same, and `claude plugin test` refuses to run there (8684185).
- A `subagentStatusLine` setting that still names the removed command gets no answer from the kit.
