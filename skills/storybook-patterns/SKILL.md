---
name: storybook-patterns
description: Storybook patterns for CSF 3, args and fn() spies, play functions, story ids, and the a11y addon. Use for *.stories.* files, .storybook/, storybook in package.json, or Storybook questions.
---

# Storybook patterns

Default assumption: Storybook 10.6 with CSF 3 stories.

- Current docs show CSF Factories (`preview.meta({...})`, `meta.story({...})`) next to plain CSF 3 objects. Match the form the project already uses.
- Adapt advice to the version in the project's `package.json` or lockfile.

## Reference files

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have.

| Before                                                                          | Read                                                     |
| ------------------------------------------------------------------------------- | -------------------------------------------------------- |
| Linking to a story, renaming a story or its title, or building a story URL      | [reference/story-ids.md](reference/story-ids.md)         |
| Writing or changing a story, its meta, args, a `play` function, or `beforeEach` | [reference/csf.md](reference/csf.md)                     |
| Configuring the a11y addon, or setting `parameters.a11y.test`                   | [reference/accessibility.md](reference/accessibility.md) |
| Reviewing Storybook code                                                        | [reference/anti-patterns.md](reference/anti-patterns.md) |
