# Client bundle

Every Client Component and its transitive imports ship to the browser; Server Components, Server Functions, and their imports stay on the server. `"use client"` sets the boundary (`server-and-client.md`).

## Audit what crosses the boundary

- Before assuming a bundle is small, run `npx next analyze` under Turbopack, or the `@next/bundle-analyzer` plugin under webpack. Open the largest block and ask whether it belongs on the client at all.
- A heavy library imported at the top of a `"use client"` file ships whole, whether the rendered JSX uses the import or not.
- A chart or rich-text editor that reaches the client through a Server Component import path with no dynamic import: `failure`.
- `lodash` (CJS) instead of `lodash-es`, or the whole package instead of one function: `warning`. Use a named import such as `import { debounce } from "lodash-es"`, or per-method imports.
- A date library namespace import (`import * as dateFns from "date-fns"`) in a Client Component: `warning`. Named imports tree-shake; namespace imports don't.
- Import icons one at a time, not the whole pack.
- In new code, use `date-fns` or `dayjs` over `moment`.

## `next/image`, `next/font`

- Use `next/image` for every raster image, never a raw `<img>`.
- Set `priority` on above-the-fold images; lazy loading is the default.
- Give each image `width` and `height`, or `fill`.
- Load fonts with `next/font/google` or `next/font/local`, never a raw `<link>` to a font CDN. Assign the generated CSS variable to `body` or a class.

## `next/dynamic`

- Defer a component until it's needed with `dynamic(() => import("./Chart"))` from `next/dynamic`.
- Add `{ ssr: false }` only for a component that can't render on the server, such as a window-bound library or a canvas visualization. With it, the page renders nothing meaningful until the client bundle hydrates.
- `next/dynamic` returns a Client Component, so its parent must be a Client Component or a Server Component that imports the dynamic factory directly.

## Tree-shaking

- Only ESM packages with side-effect-free exports tree-shake; CJS packages don't. Switch to a package's `-es` variant when one exists.
- `optimizePackageImports` in `next.config.js` rewrites a small set of known-heavy packages to barrel-free imports. Check Vercel's package-import post for the current list.
