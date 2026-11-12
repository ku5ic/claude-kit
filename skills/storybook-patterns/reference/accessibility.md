# Accessibility

- `@storybook/addon-a11y` runs axe-core against each rendered story. Axe catches up to 57% of WCAG issues, so a clean panel isn't a conformance claim.
- `parameters.a11y.test`: set it project-wide in `.storybook/preview.*`, in a component's meta, or on one story. It decides what violations do when stories run through the Vitest addon or the test-runner.

| Value     | Effect                                                                 |
| --------- | ---------------------------------------------------------------------- |
| `'off'`   | Checks don't run; the addon panel can still be used by hand            |
| `'todo'`  | Checks run; violations show as warnings in the Storybook UI            |
| `'error'` | Checks run; violations fail the test in the Storybook UI and in CLI/CI |

- CI output: only with `'error'`. `'todo'` gives nothing in CI: no error, no warning, no output.
- A known issue not fixed yet: `'todo'`.
- A story that doesn't need testing, such as one demonstrating an anti-pattern: `'off'`.
