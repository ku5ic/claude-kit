# Strictness flags and tsconfig review

## Strictness flags

The "should be on" set. Each gets a one-line behavior summary; severity for projects with the flag off is `warning` (or `failure` for `strict` itself).

- `strict` (since 2.3): umbrella for the strict-mode family. Off is a `failure`; everything else compounds on it.
- `noUncheckedIndexedAccess` (since 4.1): adds `undefined` to indexed access on records and arrays. Without it, `arr[0]` types as the element instead of `T | undefined`.
- `exactOptionalPropertyTypes` (since 4.4): an optional property `{ x?: T }` means `T | absent`, not `T | undefined`. Catches code that assigns explicit `undefined` to an optional.
- `verbatimModuleSyntax` (since 5.0): emits exactly the import/export shape you wrote; type-only imports must say so. Replaces the deprecated `importsNotUsedAsValues` and `preserveValueImports`.
- `isolatedModules`: every file must be transpilable in isolation. Required when Babel, swc, or esbuild handle transpilation; turning it on at the type layer surfaces problems at typecheck rather than at bundle time.
- `noImplicitOverride` (since 4.3): subclass methods that override must use the `override` keyword. Catches drift when the parent class renames or removes a method.
- `noFallthroughCasesInSwitch` (since 1.8): every non-empty `case` requires `break`, `return`, or `throw`. Combine with discriminated-union exhaustiveness for free.
- `forceConsistentCasingInFileNames` (since 2.0): import paths must match the on-disk casing. Default `true` since 5.0; explicit on older configs is fine.

## tsconfig in review

When reviewing a TypeScript project's `tsconfig.json`:

- `strict: true` plus the strictness-flag set above. `strict: false` is `failure`; each missing additional strictness flag is `warning`.
- `target`: matches the runtime. For Node 20+, `es2022` or newer; for browsers, leave the bundler in charge with a sane floor (`es2020` or `es2022`).
- `module` and `moduleResolution`: `"nodenext"` for both on modern Node. For bundled code, `module: "preserve"` or `"esnext"` paired with `moduleResolution: "bundler"`. Use `"node16"` for libraries publishing dual ESM/CJS. Avoid the legacy `"node"` / `"node10"` on new projects; TypeScript 7.0 removed `node10` and `classic` outright, along with `baseUrl`.
- `paths` aliases: must match the runtime resolver. A `paths` alias that compiles cleanly but the runtime cannot resolve at execution is a `failure`; bundlers, `tsx`, and runtime path mappers each need their own wiring.
- `skipLibCheck: true`: commonly enabled. Tradeoff is faster compile vs missing `.d.ts` errors from dependencies. Acceptable in most projects; not a finding on its own.
- `composite: true` on monorepo packages that participate in project references. Without it, `tsc --build` orchestration falls apart.

## Migration patterns for incremental adoption

When turning on strictness flags on an existing codebase, do it one flag at a time. Each flag has a distinct error class; mixing surfaces makes triage harder.

- Order on a typical migration: `strict` first (umbrella) -> `noUncheckedIndexedAccess` (adds `| undefined` to indexed access; many call sites need narrowing) -> `exactOptionalPropertyTypes` (catches `undefined` assignments to optional props) -> `verbatimModuleSyntax` (forces `import type` discipline).
- Before turning a flag on globally, search for the error pattern with the flag in a scratch branch. If the count is unmanageable, scope first via the language-server config or a per-file `// @ts-expect-error` annotation, then unblock the global flip.
- `noUnusedLocals` and `noUnusedParameters` are linting concerns; ESLint catches the same conditions with better integration. Leaving them off in `tsconfig.json` and on in ESLint is fine.

## References

- tsconfig reference: https://www.typescriptlang.org/tsconfig
- TypeScript handbook (compiler options): https://www.typescriptlang.org/docs/handbook/compiler-options.html
- typescript-eslint shareable configs: https://typescript-eslint.io/users/configs/
