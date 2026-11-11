# next-app-router-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

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

### Version notes

Checked: 2026-11-10 against https://www.npmjs.com/package/next, https://nextjs.org/docs/app/guides/upgrading/version-16, https://nextjs.org/docs/app/guides/package-bundling and https://nextjs.org/docs/app/api-reference/file-conventions/proxy

- 16.4 (2026-10-06, latest 16.4.6): corrected three 16.0 facts against the upgrade guide. Parallel slots without `default.js` fail the build, synchronous `cookies()`/`headers()` access is removed, and Turbopack's analyzer is `next analyze`.
- 16.3 (2026-08-03): no guidance change. Proxy runs on Node.js only; a project that needs the Edge runtime keeps `middleware.ts` (proxy docs).
- 16.0: `middleware` renamed to `proxy` (types `NextMiddleware` -> `NextProxy`, `MiddlewareConfig` -> `ProxyConfig`, `skipMiddlewareUrlNormalize` -> `skipProxyUrlNormalize`); PPR stable and default under Cache Components; one-argument `revalidateTag(tag)` deprecated, pass a `cacheLife` profile (upgrade guide; examples fixed 2026-09-26).

## reference/anti-patterns.md

### References

- Server and Client Components: https://nextjs.org/docs/app/getting-started/server-and-client-components
- Caching: https://nextjs.org/docs/app/getting-started/caching
- Server Functions: https://nextjs.org/docs/app/getting-started/mutating-data
- `dynamic` segment config: https://nextjs.org/docs/app/api-reference/file-conventions/route-segment-config#dynamic
- Error handling: https://nextjs.org/docs/app/getting-started/error-handling

## reference/caching.md

### References

- Next.js Caching (Cache Components): https://nextjs.org/docs/app/getting-started/caching
- Next.js previous caching model: https://nextjs.org/docs/app/guides/caching-without-cache-components
- `cacheComponents` config: https://nextjs.org/docs/app/api-reference/config/next-config-js/cacheComponents
- `revalidatePath`: https://nextjs.org/docs/app/api-reference/functions/revalidatePath
- `revalidateTag`: https://nextjs.org/docs/app/api-reference/functions/revalidateTag

## reference/client-bundle.md

### References

- `next/image`: https://nextjs.org/docs/app/api-reference/components/image
- `next/font`: https://nextjs.org/docs/app/api-reference/components/font
- `next/dynamic`: https://nextjs.org/docs/app/api-reference/functions/dynamic
- Vercel: package import optimization: https://vercel.com/blog/how-we-optimized-package-imports-in-next-js

## reference/routing.md

### References

- App Router structure: https://nextjs.org/docs/app/getting-started/project-structure
- Dynamic segments: https://nextjs.org/docs/app/api-reference/file-conventions/dynamic-routes
- Parallel routes: https://nextjs.org/docs/app/api-reference/file-conventions/parallel-routes
- Intercepting routes: https://nextjs.org/docs/app/api-reference/file-conventions/intercepting-routes
- `generateStaticParams`: https://nextjs.org/docs/app/api-reference/functions/generate-static-params
- `generateMetadata`: https://nextjs.org/docs/app/api-reference/functions/generate-metadata

## reference/runtime.md

### References

- Route segment config (runtime): https://nextjs.org/docs/app/api-reference/file-conventions/route-segment-config/runtime
- Proxy file convention: https://nextjs.org/docs/app/api-reference/file-conventions/proxy
- Edge runtime: https://nextjs.org/docs/app/api-reference/edge

### Claims

- Legacy (Middleware, Edge runtime), A project that needs the Edge runtime for this file keeps...: https://nextjs.org/docs/app/api-reference/file-conventions/proxy
- Environment variables: server-only vs `NEXT_PUBLIC_`, Next.js 16 removed `serverRuntimeConfig` and `publicRunti...: https://nextjs.org/docs/app/guides/upgrading/version-16

## reference/server-actions.md

### References

- Next.js Mutating Data: https://nextjs.org/docs/app/getting-started/mutating-data
- `use server` directive: https://nextjs.org/docs/app/api-reference/directives/use-server
- React Server Functions: https://react.dev/reference/rsc/server-functions
- `revalidatePath`: https://nextjs.org/docs/app/api-reference/functions/revalidatePath
- `revalidateTag`: https://nextjs.org/docs/app/api-reference/functions/revalidateTag
- `redirect`: https://nextjs.org/docs/app/api-reference/functions/redirect

## reference/server-and-client.md

### References

- Next.js Server and Client Components: https://nextjs.org/docs/app/getting-started/server-and-client-components
- React `use client`: https://react.dev/reference/rsc/use-client
- React Server Components: https://react.dev/reference/rsc/server-components

## reference/streaming.md

### References

- `loading.js`: https://nextjs.org/docs/app/api-reference/file-conventions/loading
- Caching (Cache Components, PPR default): https://nextjs.org/docs/app/getting-started/caching
- React `<Suspense>`: https://react.dev/reference/react/Suspense
