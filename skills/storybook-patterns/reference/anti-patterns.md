# Anti-patterns

Severity rubric:

- `failure`: a concrete defect or violation that should not ship.
- `warning`: a smell or pattern that compounds with other findings.
- `info`: a hardening opportunity or note, not a defect.

## `storiesOf`

`failure`. Removed in Storybook 8.0. Write CSF: a default export meta and named story exports. Source: https://github.com/storybookjs/storybook/blob/next/MIGRATION.md

## `a11y.test: 'off'` used to silence real violations

`failure`. `'off'` is for stories that don't need testing, such as an anti-pattern demo. A known issue that isn't fixed yet is `'todo'`; everything else should be `'error'`. Source: https://storybook.js.org/docs/writing-tests/accessibility-testing

## A project relying on `'todo'` as its CI gate

`warning`. `'todo'` produces nothing in CI, so violations never fail a build. Set `'error'` project-wide in `.storybook/preview.*` and mark known issues `'todo'` per story. Source: https://storybook.js.org/docs/writing-tests/accessibility-testing

## A passing a11y panel treated as WCAG conformance

`warning`. Axe catches up to 57% of WCAG issues. Keyboard behavior, focus order, and screen reader output still need the static checklist (`wcag-audit`) and manual testing. Source: https://storybook.js.org/docs/writing-tests/accessibility-testing

## A story URL or `<Story id>` built from the story's `name`

`warning`. The id comes from the export key, not `name`, so the link doesn't resolve. Build it from the title and the export. Source: https://storybook.js.org/docs/configure/user-interface/sidebar-and-urls

## A callback arg that isn't an `fn()` spy

`info`. Without `fn()`, a `play` function can't assert on the call, and the actions panel may not log it. Source: https://storybook.js.org/docs/essentials/actions
