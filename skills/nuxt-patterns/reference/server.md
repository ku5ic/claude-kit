# Server engine: Nitro

## Nitro and h3

- Nitro builds a standalone `.output/` directory that runs without modification on Node.js, Cloudflare Workers, Vercel, Netlify, Deno, and edge runtimes.
- Preset selection, storage, plugins, and routing: the `nitro` field in `nuxt.config`.
- Handlers run on h3. Return an object or array for a JSON response with the right content-type, return a promise if needed, and use h3's helpers for headers, cookies, body parsing, and validation.

## Server routes

- Files in `server/api/` become endpoints under `/api/`, with the method as a filename suffix: `users.get.ts` -> `GET /api/users`, `users.post.ts` -> `POST /api/users`, `users/[id].get.ts` -> `GET /api/users/:id`. With no suffix, the handler answers every method.
- A route param: `getRouterParam(event, "id")`. When it's missing, `throw createError({ statusCode: 400, message: "id required" })`.
- A server route that reads a body or query without validation: `failure`. Validate at the top of every handler with a schema library (Zod, Valibot): `readValidatedBody(event, schema.parse)`, or `safeParse` and `throw createError({ statusCode: 400, data: parsed.error })` on failure.

## `server/` directories

- `server/middleware/*.ts`: runs before route handlers on every request, for auth header parsing, request logging, and CORS. It mutates the event context and returns no response unless it throws.
- `server/utils/*.ts`: utilities, auto-imported into server code, with no route.
- `server/plugins/*.ts`: runs once at Nitro startup, for one-time setup such as cache warming or a DB pool.

## `runtimeConfig`

- Server-only config: a top-level `runtimeConfig` key, such as `apiSecret`, overridden by `NUXT_API_SECRET`.
- Config for client and server: a key under `runtimeConfig.public`, such as `apiBase`, overridden by `NUXT_PUBLIC_API_BASE`.
- Nuxt exposes an env var only when its `runtimeConfig` key is declared first.
- `process.env.SECRET` read in a server handler: `failure`. Use `useRuntimeConfig().apiSecret`.
- `process.env.SECRET`, or any non-`NUXT_PUBLIC_` variable, read in a Vue component or composable that ends up in the client bundle: `failure`.
