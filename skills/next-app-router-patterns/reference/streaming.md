# Streaming and Suspense

## `loading.tsx`

A `loading.tsx` (or `loading.js`) inside any segment folder wraps `page.tsx` plus the segment's nested layouts in a React `<Suspense>` boundary. While the page is rendering, the loading UI is shown immediately. The fallback is prefetched, so navigation feels instant.

`loading.tsx` does NOT wrap the segment's own `layout.tsx`, `template.tsx`, or `error.tsx`. If the layout itself blocks on uncached data, the navigation blocks until that layout is ready and the loading fallback is irrelevant. Move uncached data fetching out of `layout.tsx` into `page.tsx`, or wrap the runtime data access in its own `<Suspense>` boundary.

## `error.tsx`

`error.tsx` defines an error boundary for the segment. It must be a Client Component (uses React error-boundary primitives). It receives `error` and `reset` props; `reset()` retries rendering the segment.

A `global-error.tsx` at the app root replaces the entire UI tree on unhandled errors and must include its own `<html>` and `<body>` (it replaces the root layout).

## Manual `<Suspense>` boundaries

Beyond `loading.tsx`, wrap any async component in a `<Suspense fallback={...}>` to show a per-section fallback. Useful when a page has multiple independent async sections (header, feed, sidebar) that should stream in independently.

```tsx
import { Suspense } from "react";

export default function Page() {
  return (
    <>
      <Suspense fallback={<FeedSkeleton />}>
        <Feed />
      </Suspense>
      <Suspense fallback={<SidebarSkeleton />}>
        <Sidebar />
      </Suspense>
    </>
  );
}
```

## Partial Prerendering (PPR)

PPR ships stable in Next.js 16 and is the default rendering behavior when Cache Components is enabled (`cacheComponents: true`). The route is split into two parts:

- **Static shell**: cached and deterministic content (`"use cache"`-marked, plus pure computation), rendered at build time and served immediately.
- **Streamed content**: anything that touches runtime APIs (`cookies()`, `headers()`, `searchParams`, `params`) or uncached fetches, wrapped in `<Suspense>` and streamed at request time.

With Cache Components, runtime-data components must be wrapped in `<Suspense>` or marked `"use cache"`; otherwise the build fails with a `blocking-route` error. This is by design - it forces the streaming boundary to be explicit.

## Streaming caveats

- `notFound()` and `redirect()` must be called before any `<Suspense>` boundary renders or before any `await` that suspends. Once headers have been sent, the HTTP status cannot change.
- Some browsers buffer streamed responses below 1024 bytes; the symptom is "I don't see anything until the response is complete" on small payloads.
- Streamed responses always return HTTP 200 because headers go out before the body finishes. SEO-relevant 404s need to be detected before the response body starts (e.g. in Proxy with a rewrite to a not-found route).

## References

- `loading.js`: https://nextjs.org/docs/app/api-reference/file-conventions/loading
- Caching (Cache Components, PPR default): https://nextjs.org/docs/app/getting-started/caching
- React `<Suspense>`: https://react.dev/reference/react/Suspense
