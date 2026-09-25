---
name: next-app-router-patterns
description: Next.js App Router patterns - server/client boundary, data fetching and caching, server actions, streaming, parallel and intercepting routes, edge vs node runtime, and review checklist. Use whenever the project contains a `next.config.*`, an `app/` directory with `page`/`layout`/`route` files, or `next` in `package.json`, OR the user asks about the App Router, server and client components, server actions, or route handlers, even if Next.js is not mentioned by name.
---

# Next.js App Router patterns

Default assumption: Next.js 16 (current stable as of writing) with the App Router. Pages Router patterns are out of scope.

- Cache Components is the new caching model in 16 (opt-in via `cacheComponents: true`); the previous caching model is still supported when the flag is off.
- Verify via `package.json` or lockfile - caching model and `cacheComponents` flag differ between 15.x and 16.x.

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

## References

- Next.js docs: https://nextjs.org/docs
- App Router structure: https://nextjs.org/docs/app/getting-started/project-structure
- Caching (Cache Components): https://nextjs.org/docs/app/getting-started/caching
- Caching (previous model): https://nextjs.org/docs/app/guides/caching-without-cache-components
- Server and Client Components: https://nextjs.org/docs/app/getting-started/server-and-client-components
- Mutating Data: https://nextjs.org/docs/app/getting-started/mutating-data
- `proxy.js`: https://nextjs.org/docs/app/api-reference/file-conventions/proxy
- Next.js blog: https://nextjs.org/blog
- Lee Robinson (Vercel DX): https://leerob.io/
- Vercel Engineering: https://vercel.com/blog

## Version notes

Checked: 2026-09-12 against https://nextjs.org/blog and https://nextjs.org/docs/app/api-reference/file-conventions/proxy

- 16.3 (2026-08-03): no guidance change. Proxy runs on Node.js only; a project that needs the Edge runtime keeps `middleware.ts` (proxy docs).
- 16.0: `middleware` renamed to `proxy` (types `NextMiddleware` -> `NextProxy`, `MiddlewareConfig` -> `ProxyConfig`, `skipMiddlewareUrlNormalize` -> `skipProxyUrlNormalize`); PPR stable and default under Cache Components.
- 15.0: `fetch()` and Route Handler `GET` no longer cached by default; 15.5 gave the middleware file stable Node.js runtime support.
