# Runtime: Node, Edge, Proxy

## Proxy (formerly Middleware)

- On 16, `middleware.ts|js` is deprecated: rename it to `proxy.ts|js` and the function to `proxy`, as a default or named export. The codemod is `npx @next/codemod@canary middleware-to-proxy .`.
- Proxy runs on Node.js only, so a file that needs the Edge runtime keeps the deprecated `middleware.ts`. Middleware gained stable Node.js support in 15.5 and was Edge-only before.
- Never set `runtime` in a Proxy file; it throws.
- Put the file at the project root or in `src/`, beside `app/` or `pages/`.
- `config.matcher` narrows the paths Proxy runs on. It takes a string (`'/about/:path*'`), an array of strings, or an array of objects with `source` and optional `has`, `missing`, and `locale` for header, cookie, and query conditions.
- Keep matcher values statically analyzable; dynamic values are silently ignored.
- Proxy as the only auth gate: `failure`. A matcher that excludes a path also skips that path's Server Function calls, so authenticate inside every Server Function (`server-actions.md`).

## Environment variables

- `serverRuntimeConfig` or `publicRuntimeConfig` in a 16+ project: `failure`. Next.js 16 removed both.
- `process.env.SECRET` (no `NEXT_PUBLIC_` prefix) read in a Client Component: `failure`. The client bundle gets `""` in its place, with no error.
- Read server-only values from `process.env` in Server Components; the taint API keeps them out of Client Components.
- `NEXT_PUBLIC_*` values are inlined into the client bundle at build. To read `process.env` at request time, call `connection()` first.

## Node vs Edge per route segment

- Set a segment's runtime in `page.tsx`, `layout.tsx`, or `route.ts` with `export const runtime = "nodejs"` (the default) or `"edge"`.
- Use Edge for thin compute-bound handlers such as an auth check, a geo header, or redirect logic. It's a Web-API subset with smaller bundles and lower cold starts, no native Node modules, partial `crypto` and `process`, and no long-lived background work.
- Use Node for anything that touches a database driver, the file system, or a large npm package.
- The `revalidate` segment config isn't available with `runtime = "edge"`.
