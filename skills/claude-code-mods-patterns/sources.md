# claude-code-mods-patterns sources

The sources and version notes behind this skill, for `/meta-refresh`. Nothing links here, so a session never loads it.

LOC below is the plugin-authoring skill bundled with Claude Code 2.1.326, its `reference.md`, which the engine writes to a per-process folder when the skill loads.

## SKILL.md

### References

- Mods overview: https://code.claude.com/docs/en/plugins/mods/overview
- Mods reference: https://code.claude.com/docs/en/plugins/mods/reference
- Mods interface: https://code.claude.com/docs/en/plugins/mods/interface
- Mods events: https://code.claude.com/docs/en/plugins/mods/events
- Mods API: https://code.claude.com/docs/en/plugins/mods/api
- Mods tests: https://code.claude.com/docs/en/plugins/mods/test
- Create a mod: https://code.claude.com/docs/en/plugins/mods/create
- Changelog: https://github.com/anthropics/claude-code/blob/main/CHANGELOG.md

### Version notes

Checked: 2026-11-26 against https://registry.npmjs.org/@anthropic-ai/claude-code and https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.326

- 2.1.332 (2026-11-25), stable 2.1.326: `$` may go to an imported `function` declaration from 2.1.299, and a Button with a label or a single string child needs no key. https://code.claude.com/docs/en/plugins/mods/create, https://code.claude.com/docs/en/plugins/mods/interface
- 2.1.326 (2026-11-16) latest; stable is 2.1.311 (2026-10-26), per https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.311.
- Mods on by default from CLI 2.1.287 and Desktop 2.1.286: https://code.claude.com/docs/en/plugins/mods/overview
- 2.1.319: `$.ui.backdrop` removed for `$.theme.set` and `$.theme.clear`: https://code.claude.com/docs/en/plugins/mods/interface

## reference/hooks.md

Older guidance removed 2026-11-26; see git history.

### Claims

- The hook function, the module exports: https://code.claude.com/docs/en/plugins/mods/reference
- The hook function, every hook is: https://code.claude.com/docs/en/plugins/mods/reference
- The hook function, registering one event twice: https://code.claude.com/docs/en/plugins/mods/events
- The hook function, hooking another mod's: https://code.claude.com/docs/en/plugins/mods/events
- When a hook fails, all four: https://code.claude.com/docs/en/plugins/mods/events
- When a hook fails, in a `.catch`: LOC, the paragraph on `claude --debug` and `.catch`
- Budgets, a hook has 10 s: https://code.claude.com/docs/en/plugins/mods/reference (Limits)
- Budgets, `next.signal`: https://code.claude.com/docs/en/plugins/mods/reference
- Work that outlives a hook, all four: https://code.claude.com/docs/en/plugins/mods/api
- The prompt cache, a changed section: https://code.claude.com/docs/en/plugins/mods/reference
- The prompt cache, new context: LOC, "What a thread has sent is fixed"

## reference/drawing.md

### Claims

- Change or replace, all five: https://code.claude.com/docs/en/plugins/mods/interface
- Surfaces, `$.ui.resolve`: https://code.claude.com/docs/en/plugins/mods/interface
- Surfaces, surface-bound elements: https://code.claude.com/docs/en/plugins/mods/reference (Elements)
- Surfaces, version minimums: https://code.claude.com/docs/en/plugins/mods/reference
- Surfaces, `bold` deprecated: https://code.claude.com/docs/en/plugins/mods/reference
- Keys, both: https://code.claude.com/docs/en/plugins/mods/interface
- Redrawing, the throttle: https://code.claude.com/docs/en/plugins/mods/reference (Limits)
- Redrawing, a `$.state` read: https://code.claude.com/docs/en/plugins/mods/interface (Keep state)
- Moving the view, both: LOC, the `$.ui.view` paragraph

## reference/state.md

### Claims

- Where a value lives, all three: https://code.claude.com/docs/en/plugins/mods/interface (Keep state)
- The contract, the declaration and `types`: https://code.claude.com/docs/en/plugins/mods/interface (Keep state)
- The contract, the module imports its value types: LOC, the plugin-authoring SKILL.md "Where to write it"
- The contract, literals: https://code.claude.com/docs/en/plugins/mods/interface (Keep state)
- The contract, `StateFamily` and `Shaped`: LOC, the `$.state` paragraph under Drawing
- Reading and writing, all four: LOC, the `$.state` paragraph under Drawing

## reference/testing.md

### Claims

- claude plugin test, all: https://code.claude.com/docs/en/plugins/mods/test
- claude plugin test, `userConfig` options: LOC, the `claude plugin test` paragraph
- claude plugin validate, what it lists and `--strict`: https://code.claude.com/docs/en/plugins/mods/create
- claude plugin validate, what it refuses: https://code.claude.com/docs/en/plugins/mods/create
- claude plugin validate, type-checks: https://github.com/anthropics/claude-code/blob/main/CHANGELOG.md

## reference/anti-patterns.md

### Claims

- A guard with no `.catch`: https://code.claude.com/docs/en/plugins/mods/events
- `$.state` written while drawing: https://code.claude.com/docs/en/plugins/mods/interface (Keep state)
- Drawing state in a module variable: https://code.claude.com/docs/en/plugins/mods/interface (Keep state)
- A state reference built at run time: https://code.claude.com/docs/en/plugins/mods/interface (Keep state)
- A surface-bound element: https://code.claude.com/docs/en/plugins/mods/reference (Elements)
- `$.ui.view` from a timer: LOC, the `$.ui.view` paragraph
- A list of Buttons: https://code.claude.com/docs/en/plugins/mods/interface
- A tree that replaces a site: https://code.claude.com/docs/en/plugins/mods/interface
- `setInterval` or `setTimeout`: https://code.claude.com/docs/en/plugins/mods/api
- A system-prompt section changed mid-thread: https://code.claude.com/docs/en/plugins/mods/reference
- `Text` `bold`: https://code.claude.com/docs/en/plugins/mods/reference
