---
name: nuxt-patterns
description: Nuxt 3+ patterns - rendering modes, data fetching (useFetch, useAsyncData, $fetch), server routes, SSR-safe state, file-based routing, the Nitro runtime, and review checklist. Use whenever the project contains a `nuxt.config.*`, `nuxt` in `package.json`, or root-level `pages/`/`server/`/`composables/` directories, OR the user asks about Nuxt, its data-fetching composables, its server routes, or its SSR behavior, even if Nuxt is not mentioned by name.
---

# Nuxt patterns

Default assumption: Nuxt 4 (current stable, latest 4.5.x) with Universal Rendering (SSR).

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

## References

- Nuxt docs: https://nuxt.com/docs
- Nuxt blog (releases): https://nuxt.com/blog
- Rendering concepts: https://nuxt.com/docs/guide/concepts/rendering
- Server engine (Nitro): https://nuxt.com/docs/guide/concepts/server-engine
- Data fetching: https://nuxt.com/docs/getting-started/data-fetching
- `useState`: https://nuxt.com/docs/api/composables/use-state
- `runtimeConfig`: https://nuxt.com/docs/guide/going-further/runtime-config
- Daniel Roe (Nuxt lead): https://roe.dev/
- Anthony Fu (Vite/Nuxt overlap): https://antfu.me/
- Sebastien Chopin (Nuxt creator): https://github.com/atinux

## Version notes

Checked: 2026-09-12 against https://nuxt.com/blog and https://github.com/nuxt/nuxt/releases

- 4.5 (2026-07): `enabled` option on `useFetch`/`useAsyncData`, `useLayout` composable, stable error codes, named views by filename. No pattern here changed.
- 4.4 (2026-03): custom `useFetch`/`useAsyncData` factories (unverified on 2026-09-12; release notes not reachable).
- Nuxt 3: end-of-life 2026-07-31, final release 3.21.x. Nuxt 2: end-of-life 2024-06-30.

Nitro and h3 version independently of Nuxt; verify version-sensitive claims against the current docs.
