# Anti-patterns

Severity rubric:

- `failure`: a concrete defect or violation that should not ship.
- `warning`: a smell or pattern that compounds with other findings.
- `info`: a hardening opportunity or note, not a defect.

## Importing a server-only module into a Client Component

`failure`. Any module that imports `next/headers`, `next/server`, a DB client, or `server-only` will error at runtime when bundled into the client. Mark the boundary with `"use client"` at the highest safe node, or extract the server logic into a Server Component that passes only serializable props down.

## Caching per-user data without a cache scope

`failure`. `unstable_cache` and the Data Cache are shared across all requests by default. Caching responses that include user-specific data (session tokens, personalised content) exposes one user's data to another. Pass a user-specific `tags` or `revalidateTag` key, or opt the route segment into dynamic rendering with `export const dynamic = "force-dynamic"`.

## `"use server"` file missing from a module that calls the database

`failure`. An async function that reads or writes the database without `"use server"` at the top of the file (or inline in the function body) has no guarantee it runs on the server. In the Cache Components model, any function called from a Server Component is server-only by default, but in mixed boundaries the directive is still required for Server Functions.

## `export const dynamic = "force-dynamic"` on a layout

`warning`. Setting `force-dynamic` on a shared layout opts every page under it out of static generation and per-request caching. The cost is paid even for pages that could be fully static. Move the dynamic segment configuration to the specific `page.tsx` that actually needs it.

## Missing error boundary around a `<Suspense>` stream

`warning`. A streaming segment that throws without a sibling `error.tsx` or `<ErrorBoundary>` kills the entire page response. Every `<Suspense>` boundary that wraps async data should have a corresponding error boundary at the same or parent level.

## `"use client"` placed deep inside a subtree

`warning`. Placing `"use client"` low in the component tree does not isolate the boundary -- all imports of that component are also pulled into the client bundle. Place the directive at the outermost component that actually needs interactivity and keep server data-fetching above it.

## Accessing `cookies()` or `headers()` outside an async context

`warning`. In Next.js 15+, `cookies()` and `headers()` are async and must be awaited. Calling them synchronously returns a Promise in 16, silently returning `undefined` for most properties. Always `await cookies()` and `await headers()`.

## References

- Server and Client Components: https://nextjs.org/docs/app/getting-started/server-and-client-components
- Caching: https://nextjs.org/docs/app/getting-started/caching
- Server Functions: https://nextjs.org/docs/app/getting-started/mutating-data
- `dynamic` segment config: https://nextjs.org/docs/app/api-reference/file-conventions/route-segment-config#dynamic
- Error handling: https://nextjs.org/docs/app/getting-started/error-handling
