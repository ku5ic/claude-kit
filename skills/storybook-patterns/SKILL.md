---
name: storybook-patterns
description: Storybook patterns for CSF 3, args and fn() spies, play functions, story ids, and the a11y addon. Use for *.stories.* files, .storybook/, storybook in package.json, or Storybook questions.
---

# Storybook patterns

Default assumption: Storybook 10.6 with CSF 3 stories.

- Current docs show CSF Factories (`preview.meta({...})`, `meta.story({...})`) next to plain CSF 3 objects. Match the form the project already uses.
- Adapt advice to the version in the project's `package.json` or lockfile.

## Reference files

| File                                                     | Covers                                                               |
| -------------------------------------------------------- | -------------------------------------------------------------------- |
| [reference/story-ids.md](reference/story-ids.md)         | How an id is formed, the manager and iframe URLs, what renames break |
| [reference/csf.md](reference/csf.md)                     | Meta, args and `fn()`, `play` functions, `beforeEach`                |
| [reference/accessibility.md](reference/accessibility.md) | The a11y addon, `parameters.a11y.test`, and CI behavior              |
| [reference/anti-patterns.md](reference/anti-patterns.md) | Six review-time anti-patterns with severity calls                    |
