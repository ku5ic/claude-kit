---
name: claude-code-mods-patterns
description: Claude Code mod patterns for hooks and next, ui.render drawing, $.state, guards, and plugin tests. Load before any edit to a plugin hooks module or hooks.json, however small, or for mod questions.
---

# Claude Code mods patterns

Default assumption: Claude Code 2.1.326, the latest release. The stable channel is 2.1.311, and mods are on by default from CLI 2.1.287 and Desktop 2.1.286.

- An API added after 2.1.311 doesn't exist on the stable channel. Each reference file names the minimum version where one applies.
- Adapt advice to the version the project's users run, and to the API types the engine writes into the plugin's `.claude-plugin/types/`.

## Reference files

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                                                              | Read                                                     |
| ----------------------------------------------------------------------------------- | -------------------------------------------------------- |
| Writing or changing a hook, a matcher, a `.catch`, or work started at session start | [reference/hooks.md](reference/hooks.md)                 |
| Writing or changing a `ui.render` hook, a tree, a Button, or `$.ui.view`            | [reference/drawing.md](reference/drawing.md)             |
| Keeping a value across hooks or reloads, or declaring a `$.state` contract          | [reference/state.md](reference/state.md)                 |
| Writing or changing a mod's `*.test.ts`, or reading `claude plugin validate`        | [reference/testing.md](reference/testing.md)             |
| Writing, changing, or reviewing a mod                                               | [reference/anti-patterns.md](reference/anti-patterns.md) |
