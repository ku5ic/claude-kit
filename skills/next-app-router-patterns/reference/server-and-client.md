# Server and client boundary

- Layouts and pages are Server Components by default. They render on the server, can fetch inline in `async function Page()`, and never ship to the client; only their HTML and RSC payload do.
- `"use client"` at the top of a file marks the boundary into the client module graph. Everything that file imports joins the client bundle, transitively; the directive is per file, not per component.
- Keep the boundary deep: layouts, pages, and presentational subtrees stay on the server, and only the interactive leaf (search input, like button, modal) is a Client Component. A Client Component wrapper above a heavy library import ships that library.

## What Server Components can't do

- A file that uses state (`useState`, `useReducer`), effects (`useEffect`, `useLayoutEffect`), event handlers (`onClick`, `onChange`, `onSubmit`), browser APIs (`window`, `document`, `localStorage`), or renders a Context `<Provider>` needs `"use client"`, or that logic moves into a nested Client Component.

## What Client Components can't do

- Import a database driver.
- Read server-only secrets: `process.env.API_KEY` without the `NEXT_PUBLIC_` prefix is an empty string in client bundles.
- Be async function components; `async function Comp()` is server-only.
- Call `headers()`, `cookies()`, or other request-time server APIs.

## Serialization across the boundary

- Props from a Server Component to a Client Component must be serializable by React: primitives, plain objects, arrays, `Date`, `Map`, `Set`, typed arrays, promises (streamed via `use()`), and some other shapes.
- Functions fail at the boundary, except Server Functions, which pass by reference. Class instances and non-plain objects fail too.
- A Server Component may be the `children` of a Client Component, such as a `<Cart>` Server Component inside a `<Modal>` Client Component.

## `headers()`, `cookies()`, `server-only`

- `headers()` and `cookies()` are async and server-only. Calling them from a file that ends up in the client bundle is a build error.
- At the top of any module with secrets or server-only logic, `import "server-only"`, so a Client Component import of it fails the build.
- For `process.env` in client code, see `runtime.md`'s environment variables section.
