# Routing

## File-based routing

Routes live under `app/`. Each folder is a URL segment; special files in the folder define the segment's behavior:

- `page.tsx`: route UI for the segment.
- `layout.tsx`: shared UI that wraps the segment and its children; persists across navigations.
- `route.ts`: HTTP-method-based handler (replaces `page.tsx` in API-only segments).
- `loading.tsx`: Suspense fallback for the segment.
- `error.tsx`: error boundary for the segment.
- `not-found.tsx`: rendered when `notFound()` is thrown in the segment.
- `template.tsx`: like `layout.tsx` but re-mounts on navigation.
- `default.tsx`: parallel-route fallback.

## Dynamic segments

Folder named `[id]` becomes a dynamic segment. The page receives `params` as a Promise that resolves to the matched values:

```tsx
export default async function Page({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
}
```

`params` is a Promise in v15+ (this was a breaking change with a codemod). Catch-all (`[...slug]`) and optional catch-all (`[[...slug]]`) work the same way; the resolved type widens to `string[]`.

## Route groups

`(marketing)/` is a route group: wraps a set of routes for organization without affecting the URL. Useful for sharing a layout across a subset of routes that do not share a URL prefix.

## Parallel routes

`@slot` folders define named slots that render in parallel within the same layout. The layout receives each slot as a prop alongside `children`:

```tsx
export default function Layout({
  children,
  modal,
  sidebar,
}: {
  children: React.ReactNode;
  modal: React.ReactNode;
  sidebar: React.ReactNode;
}) {}
```

Use parallel routes for split-view dashboards, persistent modals, and conditional sub-trees that need independent loading/error states.

A `default.tsx` in each parallel slot defines the fallback rendered when a navigation does not specify a value for that slot. Without it, missing slot state can break navigation.

## Intercepting routes

`(.)folder`, `(..)folder`, `(...)folder` intercept a navigation from another part of the tree to render a different UI for the same URL - e.g. clicking a thumbnail opens a modal that overlays the gallery, while a direct visit to that URL renders the full photo page.

Combined with parallel routes, this is the standard pattern for modal routes that survive page refresh as a real URL.

## `generateStaticParams`

Returns the list of `params` values to prerender at build time for a dynamic segment:

```tsx
export async function generateStaticParams() {
  const posts = await fetch("https://...").then((r) => r.json());
  return posts.map((p) => ({ slug: p.slug }));
}
```

Combine with `dynamicParams` (route segment config) to control whether unknown params render at request time or 404.

## `generateMetadata`

Returns the page's `<head>` metadata, async, with access to `params` and `searchParams`:

```tsx
export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const post = await getPost(id);
  return { title: post.title, description: post.summary };
}
```

The returned object must be serializable. Streaming metadata is supported for crawlers that handle it; for crawlers that only see static HTML, Next.js falls back to blocking metadata before streaming the body.

## References

- App Router structure: https://nextjs.org/docs/app/getting-started/project-structure
- Dynamic segments: https://nextjs.org/docs/app/api-reference/file-conventions/dynamic-routes
- Parallel routes: https://nextjs.org/docs/app/api-reference/file-conventions/parallel-routes
- Intercepting routes: https://nextjs.org/docs/app/api-reference/file-conventions/intercepting-routes
- `generateStaticParams`: https://nextjs.org/docs/app/api-reference/functions/generate-static-params
- `generateMetadata`: https://nextjs.org/docs/app/api-reference/functions/generate-metadata
