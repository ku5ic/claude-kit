# Routing

## File-based routes

- Files in `pages/` become routes, the folder structure mirroring the URL: `pages/index.vue` -> `/`, `pages/about.vue` -> `/about`, `pages/blog/[slug].vue` -> `/blog/:slug`, `pages/blog/[...slug].vue` -> `/blog/:catchAll`, `pages/users/[[id]].vue` -> `/users` and `/users/:id`.
- A param in `pages/[slug].vue`: read it with `useRoute().params.slug`, or with `definePageMeta` for type-safe access.

## Layouts

- `layouts/default.vue` wraps every page. Another layout, such as `layouts/admin.vue`: opt in per page with `definePageMeta({ layout: "admin" })`.
- `<NuxtLayout>` and `<NuxtPage>` are the slot components; the default layout uses them implicitly.

## Route middleware

- One page only: anonymous middleware inline, `definePageMeta({ middleware: [(to) => { ... }] })`.
- Opt in per page by name: `middleware/auth.ts`, then `definePageMeta({ middleware: 'auth' })`.
- Every route navigation: `middleware/auth.global.ts`.
- Route middleware runs on both server and client. Scope a side effect with `import.meta.server` or `import.meta.client`.
- A redirect: return `navigateTo("/login")`. A 404: throw `createError({ statusCode: 404 })`.

## Route middleware vs server middleware

- Logic before a page renders that needs composables (`useState`, `useFetch`): route middleware in `middleware/`, which runs in the Vue rendering pipeline.
- Auth header parsing, request logging, CORS: server middleware in `server/middleware/`, which runs in Nitro on every server request before route handlers and has no Vue composables.
- Tell the two apart by directory.

## `definePageMeta`

- `definePageMeta` sets per-page `layout`, `middleware`, `alias` (such as `["/dashboard"]`), and `validate`.
- A param that must match a format: `validate: async (route) => /^\d+$/.test(route.params.id as string)`. It runs before the page renders; `false` produces a 404.
