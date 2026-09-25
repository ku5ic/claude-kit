# Routing

## File-based routing

Files in `pages/` become routes automatically. The folder structure mirrors the URL path:

- `pages/index.vue` -> `/`
- `pages/about.vue` -> `/about`
- `pages/blog/[slug].vue` -> `/blog/:slug` (dynamic)
- `pages/blog/[...slug].vue` -> `/blog/:catchAll` (catch-all)
- `pages/users/[[id]].vue` -> `/users` and `/users/:id` (optional param)

In `pages/[slug].vue`, access the param via `useRoute().params.slug` or `definePageMeta` for type-safe access.

## Layouts

`layouts/default.vue` wraps every page by default. Other named layouts (`layouts/admin.vue`) are opted in per page:

```vue
<script setup>
definePageMeta({ layout: "admin" });
</script>
```

`<NuxtLayout>` and `<NuxtPage>` are the slot components; the default layout uses them implicitly.

## Route middleware

Three placement options, with different lifecycles:

- **Anonymous**: `definePageMeta({ middleware: [(to) => { /* ... */ }] })`. Inline, page-scoped.
- **Named**: `middleware/auth.ts` -> `definePageMeta({ middleware: 'auth' })`. Named, opt-in per page.
- **Global**: `middleware/auth.global.ts`. Runs on every route navigation.

Middleware runs on both server and client. Use `import.meta.server` / `import.meta.client` to scope side effects:

```ts
export default defineNuxtRouteMiddleware((to) => {
  if (import.meta.server) return;
  const { user } = useAuth();
  if (!user.value) return navigateTo("/login");
});
```

`navigateTo()` returns a redirect; throwing `createError({ statusCode: 404 })` produces a 404 response.

## Server middleware vs route middleware

Two distinct concepts:

- **Route middleware** (`middleware/`): runs in the Vue rendering pipeline, before a page renders. Has access to composables (`useState`, `useFetch`).
- **Server middleware** (`server/middleware/`): runs in Nitro on every server request, before route handlers. Use for auth header parsing, request logging, CORS. Does NOT have access to Vue composables.

Mixing the two up is a common confusion; the directory tells you which is which.

## `definePageMeta`

Set per-page metadata that the framework reads at build/route time:

```vue
<script setup>
definePageMeta({
  layout: "admin",
  middleware: ["auth"],
  alias: ["/dashboard"],
  validate: async (route) => /^\d+$/.test(route.params.id as string),
});
</script>
```

`validate` runs before the page renders; returning `false` produces a 404.

## References

- Pages directory: https://nuxt.com/docs/guide/directory-structure/pages
- Layouts directory: https://nuxt.com/docs/guide/directory-structure/layouts
- Middleware directory: https://nuxt.com/docs/guide/directory-structure/middleware
- `definePageMeta`: https://nuxt.com/docs/api/utils/define-page-meta
- `navigateTo`: https://nuxt.com/docs/api/utils/navigate-to
