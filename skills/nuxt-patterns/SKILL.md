---
name: nuxt-patterns
description: Nuxt 3+ patterns for rendering modes, useFetch and useAsyncData, server routes, SSR state, and Nitro. Use for nuxt.config.*, nuxt in package.json, Nuxt app dirs, or Nuxt questions.
---

# Nuxt patterns

Default assumption: Nuxt 4 (current stable, latest 4.6.x) with Universal Rendering (SSR).

- A Nuxt 3 project: most patterns here apply. Flag that Nuxt 3 reached end-of-life on 2026-07-31.
- Before a version-sensitive call, check the installed version with `nuxt --version` or `package.json`. Minor versions can change module resolution and Nitro config.

## Reference files

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have.

| Before                                                                         | Read                                                     |
| ------------------------------------------------------------------------------ | -------------------------------------------------------- |
| Reviewing Nuxt code                                                            | [reference/anti-patterns.md](reference/anti-patterns.md) |
| Choosing a rendering mode, or setting `routeRules` or prerendering             | [reference/rendering.md](reference/rendering.md)         |
| Fetching data with `$fetch`, `useFetch`, or `useAsyncData`                     | [reference/data-fetching.md](reference/data-fetching.md) |
| Adding shared state, `useState`, a Pinia store, or a composable                | [reference/state.md](reference/state.md)                 |
| Adding a page, a layout, route middleware, or `definePageMeta`                 | [reference/routing.md](reference/routing.md)             |
| Writing a server route, Nitro or h3 code, `runtimeConfig`, or reading env vars | [reference/server.md](reference/server.md)               |
