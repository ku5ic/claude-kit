# React Testing Library

- Use `userEvent` over `fireEvent`, and always `await user.click(...)`.
- Never query by class name or CSS selector.
- Wrap only async assertions in `waitFor`, never synchronous ones.
- An async element: use `findBy*`, which returns a promise and polls already. Never wrap it in `waitFor`.
- Render components fully; no shallow rendering.
- An element you can't find by role or label points to an accessibility issue: fix the component, which usually beats a workaround such as `getByTestId`.

## Query semantics: get / query / find

| Prefix     | Element present       | Element absent             | Async retry          |
| ---------- | --------------------- | -------------------------- | -------------------- |
| `getBy*`   | returns the element   | throws a descriptive error | no                   |
| `queryBy*` | returns the element   | returns `null`             | no                   |
| `findBy*`  | resolves with element | rejects with timeout error | yes (default 1000ms) |

- Asserting presence: `screen.getByRole("button", { name: /submit/i })`. A miss throws "found none" or "found multiple".
- Asserting absence: `expect(screen.queryByText(/loading/i)).toBeNull()`, since `getBy*` would throw.
- Async: `await screen.findByText(/welcome/i)`, which polls with `getBy*` semantics.

## Full priority order

1. `getByRole`: the accessibility tree, what assistive tech sees.
2. `getByLabelText`: form fields associated with a label.
3. `getByPlaceholderText`: when a label is genuinely not available.
4. `getByText`: non-interactive content matched by visible text.
5. `getByDisplayValue`: form fields by their current value.
6. `getByAltText`: images and other media with alt attributes.
7. `getByTitle`: last semantic option; screen reader support is inconsistent.
8. `getByTestId`: escape hatch when none of the above apply.
