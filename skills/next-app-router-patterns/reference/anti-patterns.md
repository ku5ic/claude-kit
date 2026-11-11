# Anti-patterns

Severity levels: the report-format skill.

- `cookies()` or `headers()` used without `await` on 15+: `warning`. Await both; 16 removes synchronous access.
- A module that imports `next/headers`, `next/server`, a DB client, or `server-only`, pulled into a Client Component: `failure`. Put `"use client"` at the highest safe node, or move the server logic into a Server Component that passes only serializable props down.
- Per-user data (session tokens, personalised content) in `unstable_cache` or the Data Cache with no per-user scope: `failure`. Both are shared across requests by default. Pass a user-specific `tags` or `revalidateTag` key, or set `export const dynamic = "force-dynamic"` on the route segment.
- An async function that reads or writes the database, with no `"use server"` at the top of its file or its body: `failure`. Under Cache Components a function called from a Server Component is server-only by default, but a Server Function across a mixed boundary still needs the directive.
- `export const dynamic = "force-dynamic"` on a shared layout: `warning`. Move it to the `page.tsx` that needs it.
- A `<Suspense>` boundary around async data with no `error.tsx` or `<ErrorBoundary>` at the same or a parent level: `warning`.
- `"use client"` on a layout, page, or wrapper around mostly static UI: `warning`. Put it on the interactive leaf, and keep server data fetching above it.
