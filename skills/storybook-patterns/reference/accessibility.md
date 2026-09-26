# Accessibility

## The a11y addon

`@storybook/addon-a11y` runs axe-core against each rendered story. Axe catches up to 57% of WCAG issues automatically, so a clean panel is not a conformance claim. Source: https://storybook.js.org/docs/writing-tests/accessibility-testing

## `parameters.a11y.test`

This parameter can be set project-wide in `.storybook/preview.*`, in a component's meta, or on one story. It decides what violations do when stories run through the Vitest addon or the test-runner:

| Value     | Effect                                                                   |
| --------- | ------------------------------------------------------------------------ |
| `'off'`   | Checks don't run; the addon panel can still be used by hand              |
| `'todo'`  | Checks run; violations show as warnings in the Storybook UI              |
| `'error'` | Checks run; violations fail the test in the Storybook UI and in CLI/CI   |

- CI output only happens with `'error'`. With `'todo'` there's nothing in CI at all: no error, no warning, no output.
- `'todo'` is meant as a literal TODO for known issues not yet fixed.
- `'off'` is only for stories that don't need testing, such as one demonstrating an anti-pattern.

Source: https://storybook.js.org/docs/writing-tests/accessibility-testing

## Checking one story from the kit

With Storybook running, `a11y-check.sh "http://localhost:6006/iframe.html?id=<story-id>&viewMode=story"` runs axe against that story and prints a digest. `reference/story-ids.md` covers how to build the id. `/audit a11y` marks findings confirmed this way `runtime-verified`.
