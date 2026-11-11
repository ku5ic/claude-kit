# Anti-patterns

Severity levels: the report-format skill.

- Module-level state in a composable or plugin, outside `useState`: `failure`. Every SSR request in the same Nitro worker shares it. Put cross-request state in `useState`.
- `$fetch` at the top level of `<script setup>`: `failure` on SSR. Use `useFetch` or `useAsyncData`.
- `process.env` read in universal code: `failure`. Use `useRuntimeConfig().public.MY_VAR` for client-accessible config and `useRuntimeConfig().MY_SECRET` for server-only config.
- `fs`, `drizzle`, a database driver, or any `server/` module imported into a universal composable in `composables/`: `failure`. Move the logic into a `server/api/` endpoint and call it with `useFetch`.
- `useFetch` or `useAsyncData` without `lazy: true` for data the initial paint doesn't need (comments, recommendations, analytics): `warning`. Pass `lazy: true` and handle `pending` in the template.
- A module-level `ref` mixed with `useState` for shared state: `warning`. Use `useState` for state that survives navigation or is shared across components, and `ref` only for local component state.
