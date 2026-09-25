# Server and client boundary

## Default is Server

Layouts and pages are React Server Components by default. They render on the server, can fetch data inline (`async function Page()`), and produce an RSC payload that the client uses to update the DOM. Server components never ship to the client; only the resulting HTML and the RSC payload do.

## `"use client"` is a module-graph boundary

Adding `"use client"` at the top of a file marks the boundary into the client module graph. Every file imported by a `"use client"` module is part of the client bundle, transitively. The directive is set per file, not per component, and it applies to the whole import subtree.

Practical consequence: a wrapper Client Component upstream of a heavy library import pulls that library to the client. Move the boundary as deep as possible - keep layouts, pages, and presentational subtrees on the server; mark only the interactive leaf (search input, like button, modal) as client.

## What server components cannot do

- React state (`useState`, `useReducer`).
- Effects (`useEffect`, `useLayoutEffect`).
- Event handlers (`onClick`, `onChange`, `onSubmit`).
- Browser APIs (`window`, `document`, `localStorage`).
- React Context (no `<Provider>` rendering).

If any of these appear, the file needs `"use client"` or the logic moves into a Client Component nested inside.

## What client components cannot do

- Direct database access (no driver imports; the connection pool would ship to the browser).
- Reading server-only secrets (`process.env.API_KEY` without the `NEXT_PUBLIC_` prefix is replaced with an empty string in client bundles).
- Async function components (`async function Comp()` is server-only).
- `headers()`, `cookies()`, and other request-time server APIs.

## Serialization across the boundary

Props passed from a Server Component to a Client Component must be serializable by React: primitives, plain objects, arrays, `Date`, `Map`, `Set`, typed arrays, promises (for streaming via `use()`), and certain other shapes. Functions are not serializable except for Server Functions (which are passed by reference). Class instances and non-plain objects fail at the boundary.

Passing a Server Component as `children` of a Client Component is allowed and common: it lets server-rendered subtrees nest visually inside client wrappers (a `<Modal>` Client Component that wraps a `<Cart>` Server Component fetched from the database).

## `headers()`, `cookies()`, `server-only`

`headers()` and `cookies()` are async server-only APIs (they return promises that resolve to read interfaces). Calling them from a file that ends up in the client bundle is a build-time error. To enforce one-way isolation, import `server-only` at the top of any module that contains secrets or server-only logic; if it ever gets imported by a Client Component, the build fails clearly.

For `process.env`: only variables prefixed with `NEXT_PUBLIC_` are inlined into the client bundle. Unprefixed variables are replaced with the empty string in client code.

## References

- Next.js Server and Client Components: https://nextjs.org/docs/app/getting-started/server-and-client-components
- React `use client`: https://react.dev/reference/rsc/use-client
- React Server Components: https://react.dev/reference/rsc/server-components
