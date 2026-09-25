# Client bundle

The client bundle is everything that ships to the browser: every Client Component plus its transitive imports. Server Components, Server Functions, and their imports stay on the server. The boundary is set by the `"use client"` directive (see `server-and-client.md`).

## Audit what crosses the boundary

A heavy library imported at the top of a `"use client"` file pulls the whole package into the client bundle, regardless of whether the imported symbol is used in the rendered JSX. Common offenders:

- `lodash`: use `lodash-es` plus the named import, or per-method imports.
- `date-fns`: named imports tree-shake; namespace imports do not.
- Icon libraries: import per icon, not the whole pack.
- `moment`: deprecated for new code; switch to `date-fns` or `dayjs`.

Run `npx @next/bundle-analyzer` (or the equivalent for the deployment target) before assuming a bundle is small. Lee Robinson's bundle-audit pattern: open the analyzer, click the largest red rectangle, ask whether it should be on the client at all.

## `next/image`

Use `next/image` for all raster images.

- Automatic responsive `srcset` and `sizes`.
- Lazy loading by default (set `priority` for above-the-fold images).
- Format negotiation (AVIF, WebP) when the browser supports it.
- Layout-shift prevention via required `width` / `height` (or `fill`).

Raw `<img>` skips all of this; Core Web Vitals (LCP, CLS) regress measurably.

## `next/font`

Use `next/font/google` or `next/font/local` to load fonts.

- Self-hosted at build time (no external request to Google).
- `font-display: swap` is set automatically.
- A CSS variable is generated that you assign to `body` or a class.
- Eliminates the layout shift caused by web-font swap.

A raw `<link>` tag to a font CDN reintroduces both the third-party request and the layout shift.

## `next/dynamic`

Defer loading a component until it's needed:

```tsx
import dynamic from "next/dynamic";

const Chart = dynamic(() => import("./Chart"));
```

`ssr: false` opts the component out of server rendering entirely:

```tsx
const Map = dynamic(() => import("./Map"), { ssr: false });
```

Use `ssr: false` for components that genuinely cannot render on the server (window-bound libraries, canvas-based visualizations). Overuse defeats SSR; the page renders nothing meaningful until the client bundle hydrates.

`next/dynamic` returns a Client Component, so the parent must be a Client Component (or a Server Component that imports the dynamic factory directly).

## Tree-shaking notes

Tree-shaking depends on the package being shipped as ESM with side-effect-free exports. CJS packages are not tree-shaken. Many older packages ship CJS only; switching to the `-es` variant (when available) often cuts dozens of KB.

Next.js's `optimizePackageImports` config option (in `next.config.js`) can transform a small set of known-heavy packages to barrel-free imports automatically. Lee Robinson's Vercel post on this is the authoritative summary; check the current list of supported packages there.

## Common bundle mistakes

- `failure`: rendering a chart or rich-text editor inside a Server Component import path that ends up Client Component, with no dynamic import. The full bundle is downloaded on first paint even though the user may never scroll to it.
- `warning`: pulling a date library namespace import (`import * as dateFns from "date-fns"`) into a Client Component. Tree-shaking does not eliminate unused functions in namespace form.
- `warning`: importing `lodash` (CJS) instead of `lodash-es`, or importing the whole package instead of a single function (`import { debounce } from "lodash-es"` over `import _ from "lodash"`).

## References

- `next/image`: https://nextjs.org/docs/app/api-reference/components/image
- `next/font`: https://nextjs.org/docs/app/api-reference/components/font
- `next/dynamic`: https://nextjs.org/docs/app/api-reference/functions/dynamic
- Vercel: package import optimization: https://vercel.com/blog/how-we-optimized-package-imports-in-next-js
