# Vitest and Jest (shared patterns)

- Use `describe` for the unit and `it` for each behavior, nested at most 2 levels.
- Mock at the module boundary, not inside the unit under test, and prefer dependency injection.
- Use `beforeEach` state only when it's truly shared; otherwise set up explicitly in each test.
- Use `toBe` for primitives, `toEqual` for structures, and `toStrictEqual` when undefined vs missing matters.
- An async test: return the promise or use `async/await`, with no `.then` chains.
- Snapshot only stable, meaningful output. For DOM trees, use Testing Library queries.

## Choosing between Vitest and Jest

- A project with a runner: stay with it, and never change runners as a side effect of an unrelated task.
- A project that uses Vite: default to Vitest (v5.x), which has a Jest-compatible API (expectations, snapshots, mocks). It works for backend code too: it needs only the Vite transform pipeline, not a Vite app.
- A project without a Vite-based build, especially one with substantial Jest setup: default to Jest.

## MSW for HTTP mocking

- A test that fakes an HTTP response: mock at the network layer with Mock Service Worker (MSW), not at the request client.
- MSW intercepts with a Service Worker in the browser and a Node interceptor in tests. The same mock definitions work in unit, integration, and e2e tests, and in the dev server for offline development.
- More than two test files mocking HTTP: use MSW. A one-off mock in a single test: `vi.fn()` is cheaper.
