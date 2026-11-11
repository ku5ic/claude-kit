# Anti-patterns

Severity levels: the report-format skill.

- `storiesOf`, removed in Storybook 8.0: `failure`. Write CSF: a default export meta and named story exports.
- `a11y.test: 'off'` used to silence real violations: `failure`. Use `'todo'` for a known issue not fixed yet and `'error'` for everything else; `'off'` is only for stories that don't need testing.
- A project relying on `'todo'` as its CI gate: `warning`. Set `'error'` project-wide in `.storybook/preview.*` and mark known issues `'todo'` per story.
- A passing a11y panel treated as WCAG conformance: `warning`. Check keyboard behavior, focus order, and screen reader output with the static checklist (`wcag-audit`) and manual testing.
- A story URL or `<Story id>` built from the story's `name`: `warning`. Build it from the title and the export key.
- A callback arg that isn't an `fn()` spy: `info`. Assign `fn()` from `storybook/test`.
