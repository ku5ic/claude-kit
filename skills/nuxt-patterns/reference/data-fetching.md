# Data fetching

## Choosing one

- An event handler, a Server Function, or a button click: `$fetch`, Nuxt's HTTP client built on ofetch.
- A plain HTTP fetch during component setup: `useFetch(url)`, the shorthand for `useAsyncData(url, () => $fetch(url))`.
- Any other async work during component setup (a CMS, GraphQL, or local database client, several awaited steps, or control `useFetch` doesn't expose): `useAsyncData(key, fn)`.
- Wrap a `$fetch` call made during component setup in `useFetch` or `useAsyncData`.

## `useFetch` and `useAsyncData`

- Both fetch once on the server, serialize the result into the page payload, and reuse it on the client with no second request during hydration.
- `useFetch` returns `data`, `pending`, `error`, `refresh`, and `clear` refs.
- A URL that changes: pass `useFetch` a function returning the URL, or a ref. A URL change re-fetches automatically.

## Keys

- With no explicit key, both generate one unique to the call site (`useAsyncData`: the file name and line number), so the same `useFetch` URL in two components doesn't share state.
- Give calls that should share data the same explicit key, stable for one logical request and different across different requests.
- `useAsyncData('user', ...)` on a per-route page: `failure`. Use a parametrized key, `` `user-${id}` ``.

## Lazy fetching

- Data the initial navigation shouldn't wait for: `{ lazy: true }`, or `useLazyFetch` / `useLazyAsyncData`. The page renders with `pending: true`; pair it with `<Suspense>` or a `v-if="!pending"` skeleton.
