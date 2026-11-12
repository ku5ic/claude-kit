# Rendering modes

- Default: Universal Rendering. Each route renders to HTML on the server, and Vue hydrates it on the client.
- A fully authenticated app where SEO doesn't matter: SPA mode with `ssr: false` in `nuxt.config`. The browser gets an empty shell.
- Marketing pages, docs, content sites: prerender static HTML at build time with `nuxt generate`, or per route with `routeRules: { '/path': { prerender: true } }`. No server runtime is needed.
- Most of the app SSR, a few routes different: per-route `routeRules` in `nuxt.config`, before splitting into several Nuxt projects.
- A global audience: deploy the same `nuxt build` output with a Nitro preset for an edge runtime (Vercel Edge, Cloudflare Workers, Netlify Edge), so SSR runs on the CDN edge.

## `routeRules` options

| Option                           | Effect                                                                      |
| -------------------------------- | --------------------------------------------------------------------------- |
| `prerender: true`                | Static HTML generated at build time                                         |
| `ssr: false`                     | The route renders as SPA only                                               |
| `swr: 60`                        | Stale-while-revalidate caching for that many seconds; `true` caches forever |
| `isr: 60`                        | Incremental static regeneration on a CDN                                    |
| `redirect: '/new'`               | Server-side redirect                                                        |
| `cors: true`, `headers: { ... }` | Response headers                                                            |

## Render-mode mismatches

- A route with `prerender: true` that calls `useState`, `useFetch` with no SSR-safe key, or a runtime API: `failure`.
- `prerender: true` on frequently changing data with no `swr` or `isr` fallback: `warning`.
