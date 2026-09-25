# React Testing Library

- `userEvent` over `fireEvent`. Always `await user.click(...)`.
- Do not query by class name or CSS selector.
- `waitFor` around async assertions only. Do not wrap synchronous assertions.
- `findBy*` returns a promise, use for async elements. Do not combine with `waitFor`.
- No shallow rendering. Render components fully.

Accessibility check built into tests: if you cannot find an element by role or label, the component likely has an accessibility issue worth fixing, not working around.

## Query semantics: get / query / find

The three query prefixes have distinct error semantics. Choose deliberately.

| Prefix     | Element present       | Element absent             | Async retry          |
| ---------- | --------------------- | -------------------------- | -------------------- |
| `getBy*`   | returns the element   | throws a descriptive error | no                   |
| `queryBy*` | returns the element   | returns `null`             | no                   |
| `findBy*`  | resolves with element | rejects with timeout error | yes (default 1000ms) |

Practical rules:

- **Asserting presence**: `screen.getByRole("button", { name: /submit/i })`. The error on miss is informative ("found none" or "found multiple").
- **Asserting absence**: `expect(screen.queryByText(/loading/i)).toBeNull()`. `getBy*` would throw; `queryBy*` returns `null` so the negative assertion works.
- **Async**: `await screen.findByText(/welcome/i)`. Combines polling + `getBy*` semantics. Do NOT wrap a `findBy*` in `waitFor`; it polls already.

## Full priority order

The Testing Library docs document the full priority chain:

1. `getByRole` -- queries the accessibility tree; matches what assistive tech sees.
2. `getByLabelText` -- form fields associated with a label.
3. `getByPlaceholderText` -- when a label is genuinely not available.
4. `getByText` -- non-interactive content matched by visible text.
5. `getByDisplayValue` -- form fields by their current value.
6. `getByAltText` -- images and other media with alt attributes.
7. `getByTitle` -- last semantic option; screen reader support is inconsistent.
8. `getByTestId` -- escape hatch when none of the above apply.

The order is not arbitrary: it tracks the accessibility tree, which is what real users (and assistive tech) navigate. A component that cannot be found by `getByRole` is signaling that its accessibility surface is weak; fixing the component is usually better than reaching for `getByTestId`.

## References

- Testing Library queries: https://testing-library.com/docs/queries/about/
- Testing Library guiding principles: https://testing-library.com/docs/guiding-principles/
- userEvent: https://testing-library.com/docs/user-event/intro
