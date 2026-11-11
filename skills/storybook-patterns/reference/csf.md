# CSF: meta, args, play

- The default export (the meta) names the `component` and holds shared `args`, `argTypes`, `decorators`, `parameters`, and `tags`.
- Each named export is a story object that overrides `args` or adds `render`, `play`, or `beforeEach`.
- CSF Factories write the same thing as `preview.meta({...})` and `meta.story({...})`.
- A callback arg: assign `fn()` from `storybook/test`, as in `onClick: fn()`.
- A `fn()` defined outside `args`: give it a name, or it won't log to the actions panel.
- A `play` function receives `canvas`, `userEvent`, `args`, and `step`. In it, query with Testing Library queries (`canvas.getByRole`, `findByLabelText`), drive with `userEvent`, and assert with `expect`, all from `storybook/test`.
- Every `userEvent` call and every `expect` in a `play` function: `await` it.
- A mock's return value: set it in the story's `beforeEach`, as in `args.getUsers.mockResolvedValue(users)`, never inside the component or the `play` function.
