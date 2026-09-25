# Runtime: Node, Edge, Proxy

## Node vs Edge per route segment

Each segment (`page.tsx`, `layout.tsx`, `route.ts`) can declare its runtime via segment config:

```ts
export const runtime = "nodejs"; // default
// or
export const runtime = "edge";
```

Edge runtime is a constrained Web-API subset:

- Smaller bundles.
- Lower cold start.
- No native Node modules (`fs`, `crypto` is partial, `process` is partial).
- No long-lived background work.

Use Edge for thin compute-bound handlers (auth check, geo header, redirect logic). Default to Node for anything that touches a database driver, file system, or large npm package.

The `revalidate` route segment config is not available when `runtime = "edge"`.

## Proxy (formerly Middleware)

In Next.js 16, the `middleware.ts|js` file convention was deprecated and renamed to `proxy.ts|js`. The function name is `proxy` (default export or named export). A codemod migrates: `npx @next/codemod@canary middleware-to-proxy .`

```ts
import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export function proxy(request: NextRequest) {
  if (!request.cookies.has("session")) {
    return NextResponse.redirect(new URL("/login", request.url));
  }
}

export const config = {
  matcher: ["/dashboard/:path*"],
};
```

Place the file at the project root (or `src/`), at the same level as `app/` or `pages/`.

The `runtime` config option is not available in Proxy files; setting it throws. Proxy runs on the Node.js runtime only. The file convention (Middleware before v16.0, Proxy after) gained stable Node.js runtime support in v15.5, having been Edge-only previously.

### Legacy (Middleware, Edge runtime)

A project that needs the Edge runtime for this file keeps `middleware.ts`; the proxy docs (https://nextjs.org/docs/app/api-reference/file-conventions/proxy) say Edge is not supported in `proxy.ts`. Treat that as the one case where the deprecated name still applies.

## Matchers

The `config.matcher` array narrows which paths the Proxy runs on. Three forms:

- String: `'/about/:path*'`.
- Array of strings: `['/about/:path*', '/dashboard/:path*']`.
- Array of objects with `source`, optional `has`, `missing`, `locale` for header/cookie/query conditions.

Matcher values must be statically analyzable; dynamic values are silently ignored.

## Proxy vs Server Function authentication

`failure`. Do not rely on Proxy alone for auth. Server Functions are reachable as direct `POST` requests to the route where they are exported, so a Proxy matcher that excludes a path also skips Server Function calls on that path. A matcher refactor or a Server Function move can silently remove access control. Always check auth inside the Server Function itself; treat Proxy as a defense-in-depth layer, not the primary gate.

## Runtime config: `runtimeConfig.public` vs server-only

Use `runtimeConfig` in `next.config.js` (or `next.config.ts`) to expose values at runtime instead of build time:

```ts
const nextConfig = {
  serverRuntimeConfig: { mySecret: process.env.MY_SECRET },
  publicRuntimeConfig: { staticFolder: "/static" },
};
```

`serverRuntimeConfig` is server-only; `publicRuntimeConfig` is available on both. For most cases, prefer environment variables: `process.env.NEXT_PUBLIC_*` is inlined into the client bundle at build, everything else stays server-side.

`failure`: reading `process.env.SECRET` (no `NEXT_PUBLIC_` prefix) in a Client Component. The variable is replaced with `""` in the client bundle and the runtime read returns the empty string silently - no error.

## References

- Route segment config (runtime): https://nextjs.org/docs/app/api-reference/file-conventions/route-segment-config/runtime
- Proxy file convention: https://nextjs.org/docs/app/api-reference/file-conventions/proxy
- Edge runtime: https://nextjs.org/docs/app/api-reference/edge
