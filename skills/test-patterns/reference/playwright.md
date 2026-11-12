# Playwright

- Prefer role and label selectors; `page.getByRole` mirrors RTL conventions.
- Never wait for network idle; wait for the thing you need, such as a request or a visible element.
- Use a page object for a flow used in three or more tests.
- Annotate a long flow with `test.step` for better failure traces.
- Give each test its own browser context, and never share state.

## Property-based testing for invariants

- For pure-function logic with provable invariants (sorting, parsing, encoding/decoding round-trips, mathematical relations), property-based testing fits better than example-based unit tests.
- Use fast-check for JavaScript / TypeScript; it shrinks a failing case to a minimal counterexample. Use hypothesis for Python.
- It fits when you can state an invariant the function preserves for all inputs in some class: `parse(stringify(x)) === x`, `sort(xs).length === xs.length`, `encode(decode(s)) === s`.
- It doesn't fit I/O-bound code where "all inputs" is hard to enumerate, code with environment-coupled side effects, or code with an ill-defined invariant.
- Keep the example-based tests too: they document intended behavior on canonical inputs, and property tests search the input space for cases the author missed.
