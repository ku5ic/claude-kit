# Streaming and Suspense

## Partial Prerendering (PPR)

- On 16, PPR is stable and the default rendering when `cacheComponents: true` is set. Cached and deterministic content (`"use cache"`-marked, plus pure computation) renders into a static shell at build time; anything that touches `cookies()`, `headers()`, `searchParams`, `params`, or an uncached fetch streams at request time inside `<Suspense>`.
- Under Cache Components, wrap a runtime-data component in `<Suspense>` or mark it `"use cache"`, or the build fails with a `blocking-route` error.

## `loading.tsx`

- A `loading.tsx` (or `loading.js`) in a segment folder wraps `page.tsx` and the segment's nested layouts in `<Suspense>`. Its fallback shows at once and is prefetched.
- It doesn't wrap the segment's own `layout.tsx`, `template.tsx`, or `error.tsx`. A layout that blocks on uncached data blocks the navigation, so move that fetch into `page.tsx`, or wrap the runtime data access in its own `<Suspense>`.

## Error boundaries

- `error.tsx` is the segment's error boundary and must be a Client Component. It gets `error` and `reset` props, and `reset()` retries rendering the segment.
- A `global-error.tsx` at the app root replaces the root layout on unhandled errors, so it must include its own `<html>` and `<body>`.

## Manual `<Suspense>` boundaries

- When a page has independent async sections (header, feed, sidebar), wrap each in its own `<Suspense fallback={...}>` so each streams in on its own.

## Streaming caveats

- Call `notFound()` and `redirect()` before any `<Suspense>` boundary renders and before any `await` that suspends; once headers are sent, the status can't change.
- Streamed responses always return HTTP 200. Detect an SEO-relevant 404 before the body starts, such as in Proxy with a rewrite to a not-found route.
- Some browsers buffer streamed responses below 1024 bytes, so a small payload shows nothing until it completes.
