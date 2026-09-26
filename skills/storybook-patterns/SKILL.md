---
name: storybook-patterns
description: Storybook patterns - CSF 3 stories and meta, args with fn() spies, play-function interaction tests, how story ids and URLs are formed, the a11y addon's test modes, and review-time anti-patterns. Use whenever the project contains `*.stories.*` files, a `.storybook/` directory, or `storybook` in `package.json`, OR the user asks about stories, play functions, Storybook addons, or component documentation, even if Storybook is not mentioned by name.
---

# Storybook patterns

Default assumption: Storybook 10.6 with CSF 3 stories.

- CSF Factories (`preview.meta({...})` and `meta.story({...})`) appear in current docs next to plain CSF 3 objects. Match whichever form the project already uses.
- Adapt advice to the version in the project's `package.json` or lockfile, and the `versions` line in `<repo-context>` when present.

## Reference files

| File                                                     | Covers                                                                    |
| -------------------------------------------------------- | ------------------------------------------------------------------------- |
| [reference/story-ids.md](reference/story-ids.md)         | How an id is formed, the manager and iframe URLs, what renames break      |
| [reference/csf.md](reference/csf.md)                     | Meta, args and `fn()`, `play` functions, `beforeEach`                     |
| [reference/accessibility.md](reference/accessibility.md) | The a11y addon, `parameters.a11y.test`, CI behavior, and `a11y-check.sh`  |
| [reference/anti-patterns.md](reference/anti-patterns.md) | Six review-time anti-patterns with severity calls                         |

## References

- Storybook docs: https://storybook.js.org/docs
- Sidebar and URLs: https://storybook.js.org/docs/configure/user-interface/sidebar-and-urls
- Accessibility tests: https://storybook.js.org/docs/writing-tests/accessibility-testing
- Migration notes: https://github.com/storybookjs/storybook/blob/next/MIGRATION.md

## Version notes

Checked: 2026-09-26 against the npm registry and Context7 /storybookjs/storybook

- 10.6.0 (2026-09-02); 10.0.0 (2025-10-28). No change to the guidance in this pack.
- 8.0: the `storiesOf` API was removed; stories are CSF.
