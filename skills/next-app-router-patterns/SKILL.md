---
name: next-app-router-patterns
description: Next.js App Router patterns for the server/client boundary, caching, server actions, streaming, and runtimes. Use for next.config.*, app/ route files, next in package.json, or Next.js questions.
---

# Next.js App Router patterns

Default assumption: Next.js 16 (current stable as of writing) with the App Router. Pages Router patterns are out of scope.

- Cache Components is the new caching model in 16 (opt-in via `cacheComponents: true`); the previous caching model is still supported when the flag is off.
- Verify via `package.json` or lockfile: caching model and `cacheComponents` flag differ between 15.x and 16.x.

## Reference files

| File                                                             | Covers                                                                                  |
| ---------------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| [reference/anti-patterns.md](reference/anti-patterns.md)         | Severity-labeled anti-patterns: cache scoping, server/client boundary, streaming errors |
| [reference/server-and-client.md](reference/server-and-client.md) | `"use client"` propagation, server vs client constraints, serialization, `server-only`  |
| [reference/caching.md](reference/caching.md)                     | Cache Components vs previous model, `fetch` defaults, segment config, revalidation      |
| [reference/server-actions.md](reference/server-actions.md)       | Server Functions, `"use server"`, form actions, validation, revalidation                |
| [reference/streaming.md](reference/streaming.md)                 | `loading.tsx`, `<Suspense>`, error boundaries, Partial Prerendering                     |
| [reference/routing.md](reference/routing.md)                     | File conventions, dynamic segments, parallel/intercepting routes, metadata              |
| [reference/runtime.md](reference/runtime.md)                     | Node vs Edge, Proxy (renamed from Middleware in v16), runtime config                    |
| [reference/client-bundle.md](reference/client-bundle.md)         | `next/image`, `next/font`, `next/dynamic`, tree-shaking and bundle audits               |

## Out of scope

- Pages Router projects (`pages/` only, no `app/` directory). The patterns differ enough that this skill would mislead.
- React projects without Next.js (use the React skill alone).
