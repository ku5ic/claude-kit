---
name: next-app-router-patterns
description: Next.js App Router patterns for the server/client boundary, caching, server actions, streaming, and runtimes. Use for next.config.*, app/ route files, next in package.json, or Next.js questions.
---

# Next.js App Router patterns

Default assumption: Next.js 16 with the App Router.

- From 16, `cacheComponents: true` opts into Cache Components; with the flag off, the previous caching model applies.
- The caching model and the `cacheComponents` flag differ between 15.x and 16.x, so check `next` in `package.json` and the lockfile first.

## Reference files

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have.

| Before                                                                               | Read                                                             |
| ------------------------------------------------------------------------------------ | ---------------------------------------------------------------- |
| Reviewing Next.js code                                                               | [reference/anti-patterns.md](reference/anti-patterns.md)         |
| Adding `"use client"`, or passing props or data between server and client code       | [reference/server-and-client.md](reference/server-and-client.md) |
| Fetching data, setting segment config, or caching or revalidating                    | [reference/caching.md](reference/caching.md)                     |
| Writing or changing a server action or a `"use server"` function                     | [reference/server-actions.md](reference/server-actions.md)       |
| Adding `loading.tsx`, an error boundary, a `<Suspense>` boundary, or PPR             | [reference/streaming.md](reference/streaming.md)                 |
| Adding a route, a layout, `params`, a parallel or intercepting route, or metadata    | [reference/routing.md](reference/routing.md)                     |
| Writing a proxy, reading environment variables, or choosing the Node or Edge runtime | [reference/runtime.md](reference/runtime.md)                     |
| Adding `next/image`, `next/font`, or `next/dynamic`, or trimming the client bundle   | [reference/client-bundle.md](reference/client-bundle.md)         |

## Out of scope

- Pages Router projects (`pages/` only, no `app/` directory).
- React projects without Next.js; use the React skill alone.
