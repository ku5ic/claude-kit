# Server engine: Nitro

## Nitro

Nitro is the server engine that powers Nuxt's server-side runtime. It builds a standalone `.output/` directory that runs on multiple targets (Node.js, Cloudflare Workers, Vercel, Netlify, Deno, edge runtimes) without modification. The `nitro` field in `nuxt.config` configures preset selection, storage, plugins, and routing.

The runtime under the hood is **h3**: handlers return objects/arrays for automatic JSON responses, accept promises, and have helpers for headers, cookies, body parsing, and validation.

## Server routes

Files in `server/api/` become HTTP endpoints. The path mirrors the URL with the segment under `/api/`:

- `server/api/users.get.ts` -> `GET /api/users`
- `server/api/users.post.ts` -> `POST /api/users`
- `server/api/users/[id].get.ts` -> `GET /api/users/:id`

The HTTP method is encoded in the filename suffix; without a method suffix, the handler responds to all methods.

```ts
// server/api/users/[id].get.ts
export default defineEventHandler(async (event) => {
  const id = getRouterParam(event, "id");
  if (!id) throw createError({ statusCode: 400, message: "id required" });
  return await db.users.findById(id);
});
```

Returning an object serializes to JSON automatically with the right content-type header.

## Validate input at the server boundary

`failure`. A server route that accepts request body or query without validation is the same security posture as any unvalidated endpoint: shape mismatches become runtime errors, malformed input crashes handlers, and untrusted strings reach the database. Use a schema library (Zod, Valibot) at the top of every handler:

```ts
import { z } from "zod";

const UserCreateSchema = z.object({
  email: z.string().email(),
  name: z.string().min(1).max(100),
});

export default defineEventHandler(async (event) => {
  const parsed = UserCreateSchema.safeParse(await readBody(event));
  if (!parsed.success) {
    throw createError({ statusCode: 400, data: parsed.error });
  }
  return await db.users.create(parsed.data);
});
```

`readValidatedBody(event, schema.parse)` is a built-in helper for the same pattern.

## Server middleware vs server routes

`server/middleware/*.ts` runs on every incoming request before route handlers. Use it for:

- Authentication header parsing.
- Request logging.
- CORS handling at the framework boundary.

It does not return a response (unless it explicitly throws); its job is to mutate the event context for downstream handlers.

`server/utils/*.ts` exports utilities (auto-imported into server code) without producing a route.

`server/plugins/*.ts` runs once on Nitro startup; use it for one-time initialization (cache warming, DB pool setup).

## Runtime config: `runtimeConfig.public` vs server-only

Define in `nuxt.config`:

```ts
export default defineNuxtConfig({
  runtimeConfig: {
    apiSecret: "", // server-only; populated from NUXT_API_SECRET
    public: {
      apiBase: "/api", // client+server; populated from NUXT_PUBLIC_API_BASE
    },
  },
});
```

Env-var override convention: `NUXT_*` overrides the server-only key, `NUXT_PUBLIC_*` overrides the public key. Variables not declared in `runtimeConfig` first are NOT exposed; this is intentional to prevent accidental leakage.

`failure`: reading `process.env.SECRET` directly in a server handler. The variable might not be loaded; use `useRuntimeConfig().apiSecret` so Nuxt validates and merges from the configured shape.

`failure`: reading `process.env.SECRET` (or any non-`NUXT_PUBLIC_` variable) in a Vue component or composable that ends up in the client bundle. The value is undefined at runtime and the secret leak is more likely the next time the variable name is recycled into the public namespace.

## References

- Server engine concept: https://nuxt.com/docs/guide/concepts/server-engine
- Server directory: https://nuxt.com/docs/guide/directory-structure/server
- `runtimeConfig`: https://nuxt.com/docs/guide/going-further/runtime-config
- h3 framework: https://h3.unjs.io/
- Nitro: https://nitro.unjs.io/
