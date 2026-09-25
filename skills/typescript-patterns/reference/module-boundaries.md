# Module boundaries

- `import type { Foo } from "..."` for type-only imports; `export type { Foo }` for type-only re-exports. Required when `verbatimModuleSyntax` is on; recommended even without it for legibility.
- Inline form (since 4.5): `import { run, type Result } from "./mod"` mixes value and type in one statement. Safe under `verbatimModuleSyntax`.
- Project references for monorepos: `composite: true` per package, `references` array on the consumer's `tsconfig.json`. Implies `declaration: true` and disciplined `rootDir` / `include`. See <https://www.typescriptlang.org/docs/handbook/project-references.html>.
- Circular imports: detect with tooling (`madge`, `dependency-cruiser`). `tsc` happily compiles a cycle; the tool catches it before it becomes a runtime hazard. Resolve by extracting a shared module rather than reordering imports.

## Barrel-file anti-pattern

A barrel file is an `index.ts` that re-exports from many sibling modules so consumers can write `import { X, Y, Z } from "./feature"` instead of three separate paths. The pattern looks ergonomic; in practice it carries two costs.

- **Compile / bundle cost.** A bundler that imports one symbol from a barrel still has to parse and analyze every file the barrel re-exports, even when tree-shaking will eventually drop them. For published libraries shipping barrels with thousands of modules, the parse cost shows up as 200-800ms per import in dev startup. Next.js maintains an explicit `optimizePackageImports` mechanism specifically to detect and rewrite barrel imports because tree-shaking alone does not handle the dev-time cost.
- **Circular-import risk.** A barrel that re-exports two modules that themselves import each other through the barrel produces a cycle that `tsc` compiles cleanly but that breaks at runtime when the cycle resolves with a partially-initialized module. The defect surfaces as `undefined is not a function` errors at module load.

When a barrel is unavoidable (public API surface for a library), keep it shallow: one barrel at the package boundary, no deeper barrels. For first-party application code, prefer explicit per-symbol imports from the source path; the IDE handles the autocomplete.

## References

- TypeScript handbook (modules): https://www.typescriptlang.org/docs/handbook/2/modules.html
- TypeScript handbook (project references): https://www.typescriptlang.org/docs/handbook/project-references.html
- Vercel: How we optimized package imports in Next.js (barrel-file analysis): https://vercel.com/blog/how-we-optimized-package-imports-in-next-js
