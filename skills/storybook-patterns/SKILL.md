---
name: storybook-patterns
description: Storybook patterns for CSF 3, args and fn() spies, play functions, story ids, and the a11y addon. Load before any edit to a *.stories.* file or .storybook/, however small, and for Storybook questions.
---

# Storybook patterns

Default assumption: Storybook 10.6 with CSF 3 stories.

- Current docs show CSF Factories (`preview.meta({...})`, `meta.story({...})`) next to plain CSF 3 objects. Match the form the project already uses.
- Adapt advice to the version in the project's `package.json` or lockfile.

## Reference files

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                                                          | Read                                                     |
| ------------------------------------------------------------------------------- | -------------------------------------------------------- |
| Linking to a story, renaming a story or its title, or building a story URL      | [reference/story-ids.md](reference/story-ids.md)         |
| Writing or changing a story, its meta, args, a `play` function, or `beforeEach` | [reference/csf.md](reference/csf.md)                     |
| Configuring the a11y addon, or setting `parameters.a11y.test`                   | [reference/accessibility.md](reference/accessibility.md) |
| Writing, changing, or reviewing Storybook code                                  | [reference/anti-patterns.md](reference/anti-patterns.md) |
