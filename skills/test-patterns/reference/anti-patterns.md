# Anti-patterns

Severity levels: the report-format skill.

- Asserting on internal method names, state variable values, or how often a private function was called: `failure`. Assert on what the code produces: return values, rendered output, emitted events, side effects.
- An async assertion without `await`: `failure`. Await every `expect` on a Promise, every `findBy` query, and every `waitFor` call.
- A mutable variable declared once and reset in `beforeEach`: `warning`. Initialize it fresh inside each test, or use a factory function.
- `vi.mock` / `jest.mock` on every import the subject uses: `warning`. For complex coordination logic, write integration tests or tests closer to the boundary.
- A large snapshot for behavior logic: `warning`. Keep snapshots for stable serialized output and test behavior with specific assertions.
- Only happy-path tests for a function that takes user input, external data, or optional parameters: `warning`. Add at least one test each for `null`/`undefined`, an empty collection, the maximum allowed value, and the error path.
- A test that calls a real network endpoint: `failure` in CI. Mock HTTP at the network layer (MSW, `nock`, `httpretty`), not by mocking individual client methods.
