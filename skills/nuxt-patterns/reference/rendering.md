# Rendering modes

## Universal SSR is the default

Nuxt's default mode is Universal Rendering: each route renders on the server to HTML, the response is sent immediately, and Vue hydrates on the client to make it interactive. Best balance of first-paint speed and SEO with full Vue interactivity.

## SPA (client-only)

Disable server rendering globally with `ssr: false` in `nuxt.config`. The browser receives an empty shell and downloads JS to render. Useful for fully-authenticated apps where SEO does not matter and the SSR cost is not worth it.

## SSG (static)

Run `nuxt generate` (or set `routeRules: { '/path': { prerender: true } }` per route) to prerender pages to HTML at build time. Output is static; no server runtime needed. Use for marketing pages, docs, content sites.

## Hybrid via `routeRules`

`routeRules` in `nuxt.config` defines per-route rendering and caching strategy. The most useful options:

- `prerender: true`: generate static HTML at build time.
- `ssr: false`: render this route as SPA-only.
- `swr: 60` (or `true` for forever): stale-while-revalidate caching for this many seconds.
- `isr: 60`: incremental static regeneration on a CDN.
- `redirect: '/new'`: server-side redirect.
- `cors: true`, `headers: { ... }`: response headers.

```ts
// nuxt.config.ts
export default defineNuxtConfig({
  routeRules: {
    "/": { prerender: true },
    "/blog/**": { swr: 3600 },
    "/admin/**": { ssr: false },
    "/api/**": { cors: true },
  },
});
```

`routeRules` is the right tool when most of the app is SSR but specific routes need different behavior. Reach for it before splitting the app into multiple Nuxt projects.

## Edge-Side Rendering

When deployed to a Nitro preset that targets edge runtimes (Vercel Edge, Cloudflare Workers, Netlify Edge), SSR runs on the CDN edge. Cuts latency significantly for global audiences. The same `nuxt build` artifact is deployed; Nitro's preset selection handles the runtime difference.

## Render-mode mismatches

`failure`: a page that calls `useState`, `useFetch` with no SSR-safe key, or a runtime API on a route marked `prerender: true`. Prerender runs at build time without per-request context; per-request data leaks into the prerendered HTML and gets served to every visitor.

`warning`: mixing `prerender: true` with frequently-changing data and no `swr`/`isr` fallback. Static HTML stays stale until the next build; users see old content.

## References

- Rendering concepts: https://nuxt.com/docs/guide/concepts/rendering
- `routeRules`: https://nuxt.com/docs/guide/concepts/rendering#hybrid-rendering
- Nitro presets / deployment: https://nuxt.com/deploy
