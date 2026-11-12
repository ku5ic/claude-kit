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

### Left to linters

Checked: 2026-11-11 with TypeScript 7.0.2 (`tsc --strict --noEmit`, plus `--jsx react-jsx` so the probe parses) and ESLint 10.14.0 with `js.configs.recommended` from `@eslint/js` 10.0.1, against storybook and `@storybook/react` 10.6.7, on probe `.stories.tsx` and `.stories.js` files. No rule was dropped, because no default check reported a Storybook rule on every form it covers.

Not caught by defaults, so kept: `storiesOf` (tsc `TS2305` fires on the import in a `.stories.tsx` file, but ESLint is silent on the same import in a `.stories.js` file), a callback arg without `fn()`, an un-awaited `userEvent` call or `expect` in `play`, a mock's return value set in `play`, `a11y.test: 'off'`, a story URL built from `name`.

## reference/accessibility.md

### Claims

- `@storybook/addon-a11y` runs axe-core, up to 57% of WCAG issues: https://storybook.js.org/docs/writing-tests/accessibility-testing
- `parameters.a11y.test`, its values, and CI output: https://storybook.js.org/docs/writing-tests/accessibility-testing

## reference/anti-patterns.md

### Claims

- `storiesOf` removed in 8.0: https://github.com/storybookjs/storybook/blob/next/MIGRATION.md
- `a11y.test: 'off'` to silence violations, `'todo'` as a CI gate, a passing panel as conformance: https://storybook.js.org/docs/writing-tests/accessibility-testing
- A story URL or `<Story id>` built from `name`: https://storybook.js.org/docs/configure/user-interface/sidebar-and-urls
- A callback arg that isn't an `fn()` spy: https://storybook.js.org/docs/essentials/actions

## reference/csf.md

### Claims

- Meta and named story exports: https://github.com/storybookjs/storybook/blob/next/docs/_snippets/storybook-interactions-play-function.md
- Callback args as `fn()` spies, naming a `fn()` outside `args`: https://storybook.js.org/docs/essentials/actions
- `play` function context, queries, `await` on `userEvent` and `expect`: https://github.com/storybookjs/storybook/blob/next/docs/_snippets/interaction-test-fn-mock-spy.md
- Mock setup in `beforeEach`: https://github.com/storybookjs/storybook/blob/next/docs/_snippets/interaction-test-complex.md

## reference/story-ids.md

### Claims

- How an id is formed, inferred titles, `id` and `name` in the meta and story: https://storybook.js.org/docs/configure/user-interface/sidebar-and-urls
- The manager URL, `?path=/story/<story-id>`: https://storybook.js.org/docs/configure/user-interface/sidebar-and-urls
- The iframe URL, `iframe.html?id=<story-id>&viewMode=story`: https://storybook.js.org/docs/sharing/embed
- What breaks links on a rename: https://github.com/storybookjs/storybook/blob/next/code/addons/docs/docs/mdx.md
