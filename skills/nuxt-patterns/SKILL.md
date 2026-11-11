---
name: nuxt-patterns
description: Nuxt 3+ patterns for rendering modes, useFetch and useAsyncData, server routes, SSR state, and Nitro. Use for nuxt.config.*, nuxt in package.json, Nuxt app dirs, or Nuxt questions.
---

# Nuxt patterns

Default assumption: Nuxt 4 (current stable, latest 4.6.x) with Universal Rendering (SSR).

- Nuxt 3 reached end-of-life on 2026-07-31; Nuxt 3 projects are still common and most patterns here apply, but flag the EOL and the version notes below cover deltas.
- Verify the installed version via `nuxt --version` or `package.json`; minor-version deltas may affect module resolution and Nitro config.

## Reference files

| File                                                     | Covers                                                                              |
| -------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| [reference/anti-patterns.md](reference/anti-patterns.md) | Severity-labeled anti-patterns: SSR state leakage, fetch dedup, runtimeConfig       |
| [reference/rendering.md](reference/rendering.md)         | SSR, SPA, SSG, hybrid via `routeRules`, edge rendering                              |
| [reference/data-fetching.md](reference/data-fetching.md) | `useFetch`, `useAsyncData`, `$fetch`, dedup, lazy fetching                          |
| [reference/state.md](reference/state.md)                 | `useState` SSR-safety, module-level leakage, Pinia, composables                     |
| [reference/routing.md](reference/routing.md)             | File-based routes, layouts, route middleware vs server middleware, `definePageMeta` |
| [reference/server.md](reference/server.md)               | Nitro, server routes, validation, `runtimeConfig` and env-var conventions           |
