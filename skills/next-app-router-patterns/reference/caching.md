# Caching

Next.js 16 has two caching models; check which one a project uses before reasoning about its caching.

## Cache Components (16+)

- Opt in with `cacheComponents: true` in `next.config.ts`.
- Cache explicitly with `"use cache"` at the top of an async function, file, or component.
- Set time-based revalidation with `cacheLife()`, and tags with `cacheTag()` for `updateTag()` invalidation in Server Functions.
- Partial Prerendering is the default: cached and deterministic content goes into a static shell at build time, and uncached or runtime-data subtrees stream at request time inside `<Suspense>`.
- A component without `"use cache"` that touches `cookies`, `headers`, `searchParams`, or `params` must sit inside `<Suspense>`, or the build fails with a `blocking-route` error.

## Previous model (`cacheComponents` off)

- Route Handler `GET` isn't cached by default since 15.0.0-RC; before that it was static.
- `fetch()` isn't cached by default; opt in with `cache: "force-cache"` or `next: { revalidate: N }`.
- Wrap a non-`fetch` async function in `unstable_cache` with a key, tags, and `revalidate`.
- Set route-wide behavior by exporting `dynamic`, `revalidate`, or `fetchCache` from `page.tsx`, `layout.tsx`, or `route.ts`. The lowest `revalidate` across a route's layouts and page wins.

## On-demand revalidation

- On 16, call `revalidateTag(tag, profile)` with a `cacheLife` profile as the second argument, `"max"` for stale-while-revalidate, or `updateTag(tag)` in a Server Action for read-your-writes. The one-argument form is deprecated there and a TypeScript error.
- On a project pinned to 15, the one-argument `revalidateTag(tag)` is still correct, and it expires the tag immediately.
- Invalidate by tag or by path with `revalidateTag` and `revalidatePath(path)` from `next/cache`, inside Server Functions or Route Handlers.
- Under Cache Components, invalidate `cacheTag`-marked content with `updateTag(tag)`.
- `refresh()` from `next/cache` refreshes the client router and doesn't revalidate tagged data.

## Common cache mistakes

- Assuming `fetch()` is cached by default on 15+: `failure`, even when the page looks right in dev.
- A parent layout with `export const dynamic = "force-static"` over a child page that reads `cookies()`: `failure`. The page errors at build.
- `force-cache` and `no-store` on different fetches in one route: `warning`. State the intent once with the route segment `revalidate` or `dynamic`.
- Relying on `unstable_cache` long-term: `warning`. The name marks the API as unstable across releases.
