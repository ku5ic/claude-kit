# Module boundaries

- Type-only imports and re-exports: `import type { Foo }` and `export type { Foo }`. Required under `verbatimModuleSyntax` (5.0+); use them without it too.
- A value and a type from one module, on 4.5+: `import { run, type Result } from "./mod"`. It's safe under `verbatimModuleSyntax`.
- A monorepo: project references, with `composite: true` per package and a `references` array in the consumer's `tsconfig.json`.
- Circular imports: `tsc` compiles them, so detect them with `madge` or `dependency-cruiser`. Extract a shared module; never just reorder imports.

## Barrel files

- First-party application code: import each symbol from its source path, not through an `index.ts` that re-exports its siblings.
- A library's public API: one shallow barrel at the package boundary, none deeper.
- When `undefined is not a function` fires at module load, look for a cycle through a barrel.
