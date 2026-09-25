# Caching

Next.js 16 ships two coexisting caching models. Verify which one a project uses before reasoning about its caching behavior.

## Cache Components (Next.js 16+ model)

Opt in by setting `cacheComponents: true` in `next.config.ts`.

- Caching becomes explicit via the `"use cache"` directive at the top of an async function, file, or component.
- Partial Prerendering (PPR) becomes the default rendering behavior:
  - Cached and deterministic content lands in a static shell at build time.
  - Uncached or runtime-data-using subtrees stream at request time wrapped in `<Suspense>`.

```tsx
import { cacheLife, cacheTag } from "next/cache";

export default async function BlogPosts() {
  "use cache";
  cacheLife("hours");
  cacheTag("posts");

  const res = await fetch("https://api.example.com/posts");
  return res.json();
}
```

- `cacheLife()` sets time-based revalidation.
- `cacheTag()` attaches tags for `updateTag()` invalidation in Server Functions.
- Without `"use cache"`, components that touch runtime APIs (`cookies`, `headers`, `searchParams`, `params`) must be wrapped in `<Suspense>` or Next will fail the build with a `blocking-route` error.

## Previous caching model (still supported)

When `cacheComponents` is not enabled, the previous model applies:

- `fetch()` requests are **not cached by default**. Opt in with `cache: "force-cache"` or `next: { revalidate: N }`.
- Route Handlers' `GET` is **not cached by default** (changed in v15.0.0-RC; previously was static by default).
- `unstable_cache` wraps non-`fetch` async functions with a key + tags + revalidate.
- Route segment config: `dynamic`, `revalidate`, `fetchCache` exported from a `page.tsx` / `layout.tsx` / `route.ts` set the route-wide behavior. The lowest `revalidate` across a route's layouts and page wins.

```tsx
export const revalidate = 60;
export const dynamic = "auto";
```

## On-demand revalidation

`revalidateTag(tag)` and `revalidatePath(path)` from `next/cache` invalidate cached entries by tag or by path. Both are safe to call inside Server Functions and Route Handlers. With Cache Components, `updateTag(tag)` is the equivalent for `cacheTag`-marked content.

```ts
"use server";
import { revalidatePath, revalidateTag } from "next/cache";

export async function publishPost(id: string) {
  // mutate
  revalidateTag("posts");
  revalidatePath(`/posts/${id}`);
}
```

`refresh()` (from `next/cache`) refreshes the client router; it does not revalidate tagged data.

## Common cache mistakes

- `failure`: assuming `fetch()` is cached by default in Next.js 15+ / 16. It is not. Pages that depend on freshness without setting `cache` may still appear correct in dev (where pages always render on-demand) and break in production.
- `failure`: a parent layout sets `export const dynamic = "force-static"` while a child page reads `cookies()`. The route-segment config wins for the route; the page errors at build.
- `warning`: combining `force-cache` and `no-store` on different fetches in the same route. Allowed but confusing; prefer route segment `revalidate` or `dynamic` to express intent once.
- `warning`: relying on `unstable_cache` long-term. The name signals API instability across releases.

## References

- Next.js Caching (Cache Components): https://nextjs.org/docs/app/getting-started/caching
- Next.js previous caching model: https://nextjs.org/docs/app/guides/caching-without-cache-components
- `cacheComponents` config: https://nextjs.org/docs/app/api-reference/config/next-config-js/cacheComponents
- `revalidatePath`: https://nextjs.org/docs/app/api-reference/functions/revalidatePath
- `revalidateTag`: https://nextjs.org/docs/app/api-reference/functions/revalidateTag
