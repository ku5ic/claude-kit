# State management

## `useState`

- `useState(key, init)` returns a ref that's per-request on the server and shared by every consumer in that request. Its value is serialized into the page payload and rehydrated on the client.
- Pass an explicit key. The call-site key it generates otherwise is less safe.
- `init` runs once per request on the server; on the client the value comes from the payload.

## Module-level state

- A `ref` or `reactive` at module top level under Nuxt SSR: `failure`. Every request in the server process shares it. Return `useState` from the composable instead: `return useState<User | null>("currentUser", () => null)`.
- The same module-level pattern in a client-side SPA is fine.

## Pinia and composables

- State read by many components across the route tree, or needing derived getters, shared actions, or DevTools: Pinia through `@pinia/nuxt`, which handles payload serialization on server and client.
- State inside one composable used by a few components: `useState` in a composable wrapper, lighter than Pinia.
- Shared logic: a composable in `composables/`, auto-imported in components. Keep its shared state in `useState`.
