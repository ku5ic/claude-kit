---
name: next-app-router-patterns
description: Next.js App Router patterns for the server/client boundary, caching, server actions, streaming, and runtimes. Use for next.config.*, app/ route files, next in package.json, or Next.js questions.
---

# Next.js App Router patterns

Default assumption: Next.js 16 with the App Router.

- From 16, `cacheComponents: true` opts into Cache Components; with the flag off, the previous caching model applies.
- The caching model and the `cacheComponents` flag differ between 15.x and 16.x, so check `next` in `package.json` and the lockfile first.

## Reference files

| File                                                             | Covers                                                                                       |
| ---------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| [reference/anti-patterns.md](reference/anti-patterns.md)         | Seven anti-patterns with severity calls: async request APIs, cache scoping, boundaries       |
| [reference/server-and-client.md](reference/server-and-client.md) | `"use client"` propagation, what each side can't do, serialization, `server-only`            |
| [reference/caching.md](reference/caching.md)                     | Cache Components vs the previous model, `fetch` defaults, segment config, `revalidateTag`    |
| [reference/server-actions.md](reference/server-actions.md)       | Pending state, `"use server"`, auth, validation, revalidation, return values                 |
| [reference/streaming.md](reference/streaming.md)                 | Partial Prerendering, `loading.tsx`, error boundaries, `<Suspense>`, streaming caveats       |
| [reference/routing.md](reference/routing.md)                     | Slot `default.tsx`, async `params`, file conventions, parallel/intercepting routes, metadata |
| [reference/runtime.md](reference/runtime.md)                     | Proxy (renamed from Middleware in 16), environment variables, Node vs Edge                   |
| [reference/client-bundle.md](reference/client-bundle.md)         | Bundle audits, `next/image`, `next/font`, `next/dynamic`, tree-shaking                       |

## Out of scope

- Pages Router projects (`pages/` only, no `app/` directory).
- React projects without Next.js; use the React skill alone.
