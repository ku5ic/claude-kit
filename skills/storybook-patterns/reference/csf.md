# CSF: meta, args, play

## Meta and stories

The default export (the meta) names the `component` and holds shared `args`, `argTypes`, `decorators`, `parameters`, and `tags`. Each named export is a story object that overrides `args` or adds `render`, `play`, or `beforeEach`. CSF Factories write the same thing as `preview.meta({...})` and `meta.story({...})`. Source: https://github.com/storybookjs/storybook/blob/next/docs/_snippets/storybook-interactions-play-function.md

## Callback args are `fn()` spies

Assign `fn()` from `storybook/test` to callback args (`onClick: fn()`). Calls then show in the actions panel, and a `play` function can assert on them. A `fn()` defined outside `args` needs a name before it logs to the actions panel. Source: https://storybook.js.org/docs/essentials/actions

## `play` functions are interaction tests

A `play` function receives `canvas`, `userEvent`, `args`, and `step`. It queries the rendered story with Testing Library queries (`canvas.getByRole`, `findByLabelText`), drives it with `userEvent`, and asserts with `expect`, all from `storybook/test`. The docs examples `await` every `userEvent` call and every `expect`; do the same. Source: https://github.com/storybookjs/storybook/blob/next/docs/_snippets/interaction-test-fn-mock-spy.md

```ts
import { expect, fn } from 'storybook/test'

export default { component: LoginForm, args: { onSubmit: fn() } }

export const FilledForm = {
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.type(canvas.getByLabelText('Email'), 'email@provider.com')
    await userEvent.click(canvas.getByRole('button', { name: 'Log in' }))
    await expect(args.onSubmit).toHaveBeenCalled()
  },
}
```

## Mock setup goes in `beforeEach`

Set up a mock's return value in the story's `beforeEach`, for example `args.getUsers.mockResolvedValue(users)`, rather than inside the component or the `play` function. Source: https://github.com/storybookjs/storybook/blob/next/docs/_snippets/interaction-test-complex.md
