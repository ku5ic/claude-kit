# Vitest and Jest (shared patterns)

- Use `describe` for the unit, `it` for each behavior. Keep nesting to 2 levels max.
- Mock at the module boundary, not inside the unit under test. Prefer dependency injection.
- Avoid `beforeEach` state unless truly shared. Explicit setup per test reads better.
- `toBe` for primitives, `toEqual` for structures, `toStrictEqual` when undefined vs missing matters.
- Async: return the promise or use `async/await`. No `.then` chains.
- Snapshots only for stable, meaningful output. Not for DOM trees (use Testing Library queries instead).

## Choosing between Vitest and Jest

Both are healthy projects in 2026. The choice is determined by build-tool fit, not feature parity.

- **Vitest** (https://vitest.dev/): Vite-native test runner with a Jest-compatible API (expectations, snapshots, mocks). Currently in v4.x. Works for backend code too -- it does not require a Vite app, only the Vite transform pipeline. The natural default for any project that already uses Vite.
- **Jest** (https://jestjs.io/): the older, broader-ecosystem incumbent. Still actively maintained. Many React ecosystem tools default to Jest config out of the box. The natural default for projects without a Vite-based build, especially ones with substantial existing Jest setup.

Migration between the two is mostly find-and-replace because the API shapes overlap. The skill recommends staying with whichever the project uses; do not introduce a runner change as a side effect of an unrelated task.

## MSW for HTTP mocking

When a test needs to fake an HTTP response, mock at the network layer rather than at the request client.

- **Mock Service Worker (MSW, https://mswjs.io/)**: intercepts at the Service Worker layer (browser) or via a Node interceptor (test environment). The same mock definitions work in unit tests, integration tests, and e2e, plus in the dev server for offline development.
- The reason to prefer MSW over `vi.fn()` / `jest.fn()` mocks of `fetch` or `axios`: client-level mocks break when you swap the client. MSW mocks survive a refactor from `fetch` to `axios` to `ky` because the contract is "what the network returns," not "what `fetch.mockReturnValue` is."

Reach for MSW the moment a project has more than two test files mocking HTTP. For one-off mocks in a single test, `vi.fn()` is cheaper.

## References

- Vitest: https://vitest.dev/
- Jest: https://jestjs.io/
- Jest matchers: https://jestjs.io/docs/expect
- MSW: https://mswjs.io/
