# Anti-patterns

Severity rubric:

- `failure`: a concrete defect or violation that should not ship.
- `warning`: a smell or pattern that compounds with other findings.
- `info`: a hardening opportunity or note, not a defect.

## Testing implementation details instead of behavior

`failure`. Asserting on internal method names, state variable values, or the number of times a private function was called couples the test to the implementation. When the implementation changes without changing behavior, the test breaks. Test what the code produces (return values, rendered output, emitted events, side effects) not how it produces it.

## Async test without `await` / `waitFor`

`failure`. An async assertion that is not awaited resolves after the test ends. The test exits with no assertion checked and reports a false pass. Every `expect` on a Promise, every `findBy` query, and every `waitFor` call must be awaited.

## Shared mutable state in `beforeEach`

`warning`. Declaring a mutable variable once and resetting it in `beforeEach` works until a test mutates it mid-run. If an earlier test modifies the shared variable before `beforeEach` runs for the next test, state bleeds across tests. Prefer fresh initialization inside each test, or use factory functions.

## Mocking so many modules that the test no longer covers the subject

`warning`. When `vi.mock` / `jest.mock` blankets every import the subject uses, the test only validates that the subject calls its dependencies -- not that the full behavior is correct. Integration tests and tests closer to the boundary are more reliable than heavily mocked unit tests for complex coordination logic.

## Large snapshot tests for behavior logic

`warning`. Snapshot tests catch unintended changes but the assertion ("this output is the same as last time") is not meaningful for logic tests. A large snapshot that breaks on every markup change produces alert fatigue and gets committed without review. Use snapshots for stable serialized output only; test behavior with specific assertions.

## Missing boundary cases

`warning`. A test suite that only covers the happy path misses the bugs that actually ship. For any function that accepts user input, external data, or optional parameters: add at least one test for `null`/`undefined`, one for empty collection, one for the maximum allowed value, and one for the error path.

## Tests that call external network endpoints

`failure` in CI. Tests that make real network calls are flaky (rate limits, outages, latency), slow, and costly. Mock HTTP at the network layer (MSW, `nock`, `httpretty`) rather than mocking individual client methods, so the real request serialization is exercised.

## References

- Testing Library guiding principles: https://testing-library.com/docs/guiding-principles/
- Kent C. Dodds (avoid testing implementation details): https://kentcdodds.com/blog/testing-implementation-details
- Vitest async handling: https://vitest.dev/guide/common-errors
- MSW: https://mswjs.io/
