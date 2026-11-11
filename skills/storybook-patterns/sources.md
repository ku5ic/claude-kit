# storybook-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- Storybook docs: https://storybook.js.org/docs
- Sidebar and URLs: https://storybook.js.org/docs/configure/user-interface/sidebar-and-urls
- Accessibility tests: https://storybook.js.org/docs/writing-tests/accessibility-testing
- Migration notes: https://github.com/storybookjs/storybook/blob/next/MIGRATION.md

### Version notes

Checked: 2026-11-10 against the npm registry and https://storybook.js.org/docs

- 10.6.7 (2026-11-10) current; 11.0.0 is in beta (beta.1, 2026-11-09), not covered. No change to the guidance in this pack.
- 10.6.0 (2026-09-02); 10.0.0 (2025-10-28).
- 8.0: the `storiesOf` API was removed; stories are CSF.

## reference/accessibility.md

### Claims

- The a11y addon, `@storybook/addon-a11y` runs axe-core against each render...: https://storybook.js.org/docs/writing-tests/accessibility-testing
- `parameters.a11y.test`: https://storybook.js.org/docs/writing-tests/accessibility-testing

## reference/anti-patterns.md

### Claims

- `storiesOf`, `failure`. Removed in Storybook 8.0. Write CSF: a default...: https://github.com/storybookjs/storybook/blob/next/MIGRATION.md
- `a11y.test: 'off'` used to silence real violations, `failure`. `'off'` is for stories that don't need testing...: https://storybook.js.org/docs/writing-tests/accessibility-testing
- A project relying on `'todo'` as its CI gate, `warning`. `'todo'` produces nothing in CI, so violations...: https://storybook.js.org/docs/writing-tests/accessibility-testing
- A passing a11y panel treated as WCAG conformance, `warning`. Axe catches up to 57% of WCAG issues. Keyboard...: https://storybook.js.org/docs/writing-tests/accessibility-testing
- A story URL or `<Story id>` built from the story's `name`, `warning`. The id comes from the export key, not `name`,...: https://storybook.js.org/docs/configure/user-interface/sidebar-and-urls
- A callback arg that isn't an `fn()` spy, `info`. Without `fn()`, a `play` function can't assert on...: https://storybook.js.org/docs/essentials/actions

## reference/csf.md

### Claims

- Meta and stories, The default export (the meta) names the `component` and h...: https://github.com/storybookjs/storybook/blob/next/docs/_snippets/storybook-interactions-play-function.md
- Callback args are `fn()` spies, Assign `fn()` from `storybook/test` to callback args (`on...: https://storybook.js.org/docs/essentials/actions
- `play` functions are interaction tests, A `play` function receives `canvas`, `userEvent`, `args`,...: https://github.com/storybookjs/storybook/blob/next/docs/_snippets/interaction-test-fn-mock-spy.md
- Mock setup goes in `beforeEach`, Set up a mock's return value in the story's `beforeEach`,...: https://github.com/storybookjs/storybook/blob/next/docs/_snippets/interaction-test-complex.md

## reference/story-ids.md

### Claims

- How an id is formed, Storybook builds a story's id from the component title an...: https://storybook.js.org/docs/configure/user-interface/sidebar-and-urls
- How an id is formed: https://storybook.js.org/docs/configure/user-interface/sidebar-and-urls
- URLs, In the Storybook UI: `?path=/story/<story-id>`, e.g. `htt...: https://storybook.js.org/docs/configure/user-interface/sidebar-and-urls
- URLs, The story alone, without the UI around it: `iframe.html?i...: https://storybook.js.org/docs/sharing/embed
- What breaks links, Renaming an export, a title, or the directory an auto-tit...: https://github.com/storybookjs/storybook/blob/next/code/addons/docs/docs/mdx.md
