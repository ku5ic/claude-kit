# Data fetching

Three primitives, with overlapping but distinct responsibilities.

## `$fetch` (the underlying transport)

`$fetch` is Nuxt's HTTP client (built on ofetch). Use it directly inside event handlers, Server Functions, and any case that does not need SSR-aware behavior:

```ts
async function publish() {
  await $fetch("/api/posts", { method: "POST", body: post });
}
```

`$fetch` alone has no server/client deduplication and no navigation-prevention. Calling it during component setup outside `useFetch` / `useAsyncData` causes the request to fire on the server AND again on the client during hydration.

## `useFetch` (the SSR-safe HTTP wrapper)

`useFetch(url, options)` wraps `$fetch` with SSR-aware behavior:

- Fetches once on the server during SSR; the result is serialized into the page payload.
- On the client, the payload is reused; no second request is fired during hydration.
- Provides `data`, `pending`, `error`, `refresh`, `clear` refs.

```vue
<script setup>
const { data: posts, pending, error } = await useFetch("/api/posts");
</script>
```

The first argument can be a string, a function returning a string, or a ref. URL changes trigger a re-fetch automatically.

## `useAsyncData` (the SSR-safe arbitrary-async wrapper)

`useAsyncData(key, fn)` does the same SSR-aware orchestration but accepts any async function, not just an HTTP call. Use it when:

- The data source is a CMS client, GraphQL client, or local database call (anything not HTTP).
- The data depends on multiple awaited steps.
- You need finer control than `useFetch` exposes.

```ts
const { data: post } = await useAsyncData(`post-${slug}`, () =>
  cmsClient.getPost(slug),
);
```

The Nuxt docs are explicit: `useFetch(url)` is sugar for `useAsyncData(url, () => $fetch(url))`. Prefer `useFetch` for plain HTTP fetches; reach for `useAsyncData` when the work is not a single HTTP call.

## Choosing between them

- **HTTP fetch tied to a URL** -> `useFetch`.
- **Anything else async during component setup** -> `useAsyncData`.
- **Event handler, Server Function, button click** -> `$fetch`.

## Keys matter for dedup

Both `useFetch` and `useAsyncData` use a unique key to dedup requests across the SSR/hydration boundary. `useFetch` derives the key from the URL automatically; `useAsyncData` requires an explicit key. The key must be stable for the same logical request and unique across different requests.

`failure`: `useAsyncData('user', ...)` on a per-route page. Every route reuses the same key, so the result of the first navigation persists across subsequent routes. Use a parametrized key: `` `user-${id}` ``.

## Lazy fetching

Add `{ lazy: true }` (or use `useLazyFetch` / `useLazyAsyncData`) to skip blocking the initial navigation. The page renders with `pending: true`, and the data fills in when ready. Pair with `<Suspense>` or a `v-if="!pending"` skeleton.

## References

- Data fetching guide: https://nuxt.com/docs/getting-started/data-fetching
- `useFetch`: https://nuxt.com/docs/api/composables/use-fetch
- `useAsyncData`: https://nuxt.com/docs/api/composables/use-async-data
- `$fetch` / ofetch: https://nuxt.com/docs/api/utils/dollarfetch
