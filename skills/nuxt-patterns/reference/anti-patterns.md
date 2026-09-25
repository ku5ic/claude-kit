# Anti-patterns

Severity rubric:

- `failure`: a concrete defect or violation that should not ship.
- `warning`: a smell or pattern that compounds with other findings.
- `info`: a hardening opportunity or note, not a defect.

## Module-level state in a composable or plugin

`failure`. Variables declared at module level (outside `useState`) are shared across all SSR requests in the same Nitro worker process. User A's state bleeds into User B's response. All cross-request state must go through `useState`, which is request-scoped on the server and reactive on the client.

## Calling `$fetch` directly in a component body

`failure` on SSR. `$fetch` inside `<script setup>` at the top level runs on every request without deduplication, waterfall-fetching on the server, then fetching again on the client after hydration. Use `useFetch` or `useAsyncData` so the result is serialized into the page payload and rehydrated without a second network call.

## Reading `process.env` directly in universal code

`failure`. `process.env` is stripped by Vite/Nitro for client bundles; values not explicitly exposed via `runtimeConfig` become `undefined` at runtime in the browser. Use `useRuntimeConfig().public.MY_VAR` for client-accessible config and `useRuntimeConfig().MY_SECRET` for server-only config.

## Importing server-only code into a universal composable

`failure`. Composables in `composables/` run on both server and client. Importing `fs`, `drizzle`, database drivers, or any `server/` module into a universal composable crashes the client bundle. Move server-only logic into `server/api/` endpoints and call them via `useFetch`.

## Missing `lazy: true` on non-critical fetches

`warning`. `useFetch` and `useAsyncData` block navigation by default -- the page does not render until the fetch resolves. For data that is not required for the initial paint (comments, recommendations, analytics), pass `lazy: true` and handle the `pending` state in the template. Blocks that delay perceived performance are flagged here.

## Mixing `useState` and raw `ref` for shared state

`warning`. `ref` declared at the module level is shared across all server requests (same issue as plain module state). Even on the client, a module-level `ref` is not reset between navigations in the same Nuxt app instance. Use `useState` for any state that must survive navigation or be shared across components, and `ref` only for local component state.

## References

- `useState` docs: https://nuxt.com/docs/api/composables/use-state
- `useFetch` docs: https://nuxt.com/docs/api/composables/use-fetch
- `runtimeConfig`: https://nuxt.com/docs/guide/going-further/runtime-config
- Server routes: https://nuxt.com/docs/guide/directory-structure/server
- Data fetching guide: https://nuxt.com/docs/getting-started/data-fetching
