# Playwright

- `page.getByRole` mirrors RTL conventions. Prefer role and label selectors.
- Wait for network idle is an anti pattern. Wait for the thing you actually need (a request, a visible element).
- Page object pattern for flows used in three or more tests.
- Use `test.step` to annotate long flows for better failure traces.
- One browser context per test. Do not share state.

## Property-based testing for invariants

For pure-function logic with provable invariants (sorting, parsing, encoding/decoding round-trips, mathematical relations), property-based testing is a stronger fit than example-based unit tests.

- **fast-check** (https://github.com/dubzzz/fast-check) for JavaScript / TypeScript. QuickCheck-inspired, actively maintained, generates random inputs that satisfy the property declaration and shrinks failing cases to a minimal counterexample.
- **hypothesis** (https://hypothesis.readthedocs.io/en/latest/) for Python. The canonical property-based library. Quoted: "you write tests which should pass for all inputs in whatever range you describe, and let Hypothesis randomly choose which of those inputs to check -- including edge cases you might not have thought about."

The fit-check: can you state an invariant the function should preserve for all inputs in some class? Examples that fit: `parse(stringify(x)) === x` for round-trips; `sort(xs).length === xs.length` for sort invariants; `encode(decode(s)) === s` for codec correctness. Examples that do not fit: I/O-bound code where the "all inputs" set is hard to enumerate, code with environment-coupled side effects, code where the invariant is ill-defined.

Property tests do not replace example-based tests. They complement them: example tests document the intended behavior on canonical inputs; property tests stress the function across the input space looking for cases the author did not consider.

## References

- Playwright: https://playwright.dev/
- Playwright locators: https://playwright.dev/docs/locators
- fast-check: https://github.com/dubzzz/fast-check
- hypothesis: https://hypothesis.readthedocs.io/en/latest/
