# Routing

- On 16, give every parallel slot a `default.tsx`, or the build fails. It renders when a navigation sets no value for that slot.
- On 15+, a page gets `params` as a Promise: type it `Promise<{ id: string }>` and `await params`. Catch-all `[...slug]` and optional catch-all `[[...slug]]` resolve to `string[]`.

## File conventions

Each folder under `app/` is a URL segment, and its special files define the segment:

- `page.tsx`: the segment's route UI.
- `layout.tsx`: shared UI around the segment and its children, kept across navigations.
- `template.tsx`: a layout that re-mounts on navigation.
- `route.ts`: an HTTP-method handler, in place of `page.tsx` in an API-only segment.
- `loading.tsx`: the segment's Suspense fallback.
- `error.tsx`: the segment's error boundary.
- `not-found.tsx`: renders when `notFound()` is thrown in the segment.
- `default.tsx`: a parallel slot's fallback.

## Segments and groups

- A folder named `[id]` is a dynamic segment.
- A route group such as `(marketing)/` shares a layout across routes with no common URL prefix, and leaves the URL alone.
- `@slot` folders are named slots rendered in parallel in one layout, which gets each slot as a prop beside `children`. Use them for split-view dashboards, persistent modals, and sub-trees that need their own loading and error states.
- `(.)folder`, `(..)folder`, and `(...)folder` intercept a navigation from elsewhere in the tree to render different UI for the same URL, such as a photo modal over the gallery while a direct visit renders the full photo page.
- For a modal route that survives a refresh as a real URL, combine an intercepting route with a parallel route.

## Static params and metadata

- To prerender a dynamic segment at build time, return its `params` values from `generateStaticParams`. Set `dynamicParams` to choose whether unknown params render at request time or 404.
- `generateMetadata` is async, gets `params` and `searchParams`, and returns the page's `<head>` metadata as a serializable object.
- Next.js streams metadata to crawlers that handle it, and blocks on metadata before the body for crawlers that only see static HTML.
