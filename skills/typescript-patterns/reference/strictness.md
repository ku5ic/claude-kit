# Strictness flags and tsconfig review

## Flags that should be on

| Since | Flag                               | Effect                                                                                                                            |
| ----- | ---------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| 5.0   | `verbatimModuleSyntax`             | Emits imports and exports as written; type-only imports must say so. Replaces `importsNotUsedAsValues` and `preserveValueImports` |
| 4.4   | `exactOptionalPropertyTypes`       | `{ x?: T }` means `T` or absent, so assigning `undefined` to it errors                                                            |
| 4.3   | `noImplicitOverride`               | An overriding method needs the `override` keyword                                                                                 |
| 4.1   | `noUncheckedIndexedAccess`         | Adds `undefined` to indexed access on arrays and records, so `arr[0]` is `T` or `undefined`                                       |
| 2.3   | `strict`                           | The strict-mode family; the other flags build on it                                                                               |
| 2.0   | `forceConsistentCasingInFileNames` | Import paths match on-disk casing. Default `true` from 5.0; set it explicitly below that                                          |
| 1.8   | `noFallthroughCasesInSwitch`       | A non-empty `case` needs `break`, `return`, or `throw`                                                                            |
|       | `isolatedModules`                  | Every file transpiles alone. Set it when Babel, swc, or esbuild transpile                                                         |

## tsconfig in review

- `moduleResolution` `node`/`node10` or `classic`, or `baseUrl`: gone in 7.0. Never on a new project.
- `strict: false`: `failure`. Each flag above that's off: `warning`.
- `target`: match the runtime. Node 20+: `es2022` or newer. Browsers: leave it to the bundler, with an `es2020` or `es2022` floor.
- `module` and `moduleResolution` on modern Node: `"nodenext"` for both. Bundled code: `module: "preserve"` or `"esnext"` with `moduleResolution: "bundler"`. A library publishing dual ESM/CJS: `"node16"`.
- A `paths` alias the runtime can't resolve: `failure`. Bundlers, `tsx`, and runtime path mappers each need their own wiring.
- `skipLibCheck: true`: not a finding on its own.

## Turning flags on in an existing codebase

- One flag at a time, in this order: `strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `verbatimModuleSyntax`.
- Before flipping a flag globally, count its errors on a scratch branch. If there are too many, scope it first through the language-server config or per-file `// @ts-expect-error`.
- `noUnusedLocals` and `noUnusedParameters`: leave them off in `tsconfig.json` and let ESLint catch unused code.
