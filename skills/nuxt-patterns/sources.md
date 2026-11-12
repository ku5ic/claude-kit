# nuxt-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

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

### Version notes

Checked: 2026-11-10 against https://www.npmjs.com/package/nuxt, https://nuxt.com/docs/4.x/api/composables/use-fetch, https://nuxt.com/docs/4.x/api/composables/use-async-data and https://nuxt.com/docs/4.x/getting-started/routing

- 4.6.4 (2026-11-02): corrected the key rule. `useFetch` and `useAsyncData` both auto-generate a call-site key; neither derives it from the URL or requires one.
- 4.5 (2026-07): `enabled` option on `useFetch`/`useAsyncData`, `useLayout` composable, stable error codes, named views by filename. No pattern here changed.
- Nuxt 3: end-of-life 2026-07-31, final release 3.21.11 (2026-08-05). Nuxt 2: end-of-life 2024-06-30.

Nitro and h3 version independently of Nuxt; verify version-sensitive claims against the current docs.

### Left to linters

Checked: 2026-11-11 with TypeScript 7.0.2 (`tsc --strict --noEmit`, no extra flags, Nuxt auto-imports declared as ambient stubs) and ESLint 10.14.0 with `@eslint/js` 10.0.1 `js.configs.recommended`, no plugins, on a probe directory. No rule was dropped, since no default check fired on any form a rule here targets.

ESLint `no-undef` fired on every auto-imported name (`useFetch`, `$fetch`, `ref`, `defineEventHandler`) and on `process` only because the probe declared no globals, so it catches none of the rules here. ESLint core skips a `.vue` file outright ("File ignored because no matching configuration was supplied").

Not caught by defaults, so kept: a module-level `ref` in a composable, `$fetch` at the top of `<script setup>`, `process.env` in universal code or a server handler, `fs` imported into a composable, `useFetch` with no `lazy: true`, a static `useAsyncData('user', ...)` key on a per-route page, `readBody` with no validation.

## reference/anti-patterns.md

### References

- `useState` docs: https://nuxt.com/docs/api/composables/use-state
- `useFetch` docs: https://nuxt.com/docs/api/composables/use-fetch
- `runtimeConfig`: https://nuxt.com/docs/guide/going-further/runtime-config
- Server routes: https://nuxt.com/docs/guide/directory-structure/server
- Data fetching guide: https://nuxt.com/docs/getting-started/data-fetching

## reference/data-fetching.md

### References

- Data fetching guide: https://nuxt.com/docs/getting-started/data-fetching
- `useFetch`: https://nuxt.com/docs/api/composables/use-fetch
- `useAsyncData`: https://nuxt.com/docs/api/composables/use-async-data
- `$fetch` / ofetch: https://nuxt.com/docs/api/utils/dollarfetch

## reference/rendering.md

### References

- Rendering concepts: https://nuxt.com/docs/guide/concepts/rendering
- `routeRules`: https://nuxt.com/docs/guide/concepts/rendering#hybrid-rendering
- Nitro presets / deployment: https://nuxt.com/deploy

## reference/routing.md

### References

- Pages directory: https://nuxt.com/docs/guide/directory-structure/pages
- Layouts directory: https://nuxt.com/docs/guide/directory-structure/layouts
- Middleware directory: https://nuxt.com/docs/guide/directory-structure/middleware
- `definePageMeta`: https://nuxt.com/docs/api/utils/define-page-meta
- `navigateTo`: https://nuxt.com/docs/api/utils/navigate-to

## reference/server.md

### References

- Server engine concept: https://nuxt.com/docs/guide/concepts/server-engine
- Server directory: https://nuxt.com/docs/guide/directory-structure/server
- `runtimeConfig`: https://nuxt.com/docs/guide/going-further/runtime-config
- h3 framework: https://h3.unjs.io/
- Nitro: https://nitro.unjs.io/

## reference/state.md

### References

- `useState`: https://nuxt.com/docs/api/composables/use-state
- State management guide: https://nuxt.com/docs/getting-started/state-management
- Pinia for Nuxt: https://pinia.vuejs.org/ssr/nuxt.html
- Composables auto-import: https://nuxt.com/docs/guide/directory-structure/composables
