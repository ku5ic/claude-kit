---
name: nuxt-patterns
description: Nuxt 3+ patterns for rendering modes, useFetch and useAsyncData, server routes, SSR state, and Nitro. Use for nuxt.config.*, nuxt in package.json, Nuxt app dirs, or Nuxt questions.
---

# Nuxt patterns

Default assumption: Nuxt 4 (current stable, latest 4.6.x) with Universal Rendering (SSR).

- A Nuxt 3 project: most patterns here apply. Flag that Nuxt 3 reached end-of-life on 2026-07-31.
- Before a version-sensitive call, check the installed version with `nuxt --version` or `package.json`. Minor versions can change module resolution and Nitro config.

## Reference files

| File                                                     | Covers                                                                                                              |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| [reference/anti-patterns.md](reference/anti-patterns.md) | Six anti-patterns with severity calls: module-level state, `$fetch` in setup, `process.env`, server imports, `lazy` |
| [reference/rendering.md](reference/rendering.md)         | SSR, SPA, SSG, hybrid via `routeRules`, edge rendering, prerender mismatches                                        |
| [reference/data-fetching.md](reference/data-fetching.md) | `$fetch`, `useFetch`, `useAsyncData`, keys and dedup, lazy fetching                                                 |
| [reference/state.md](reference/state.md)                 | `useState` SSR-safety, module-level leakage, Pinia, composables                                                     |
| [reference/routing.md](reference/routing.md)             | File-based routes, layouts, route middleware vs server middleware, `definePageMeta`                                 |
| [reference/server.md](reference/server.md)               | Nitro and h3, server routes, validation, `server/` directories, `runtimeConfig` and env vars                        |
