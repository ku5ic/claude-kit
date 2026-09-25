---
name: typescript-patterns
description: TypeScript patterns - strictness flags, type-safety anti-patterns, and a review checklist covering any vs unknown, satisfies, branded types, discriminated unions, generic constraints, and module boundaries. Use whenever the project contains `.ts`/`.tsx` files, a `tsconfig*.json`, or `typescript` in `package.json`, OR the user asks about TypeScript, its type system, or its compiler config, even if "TypeScript" is not mentioned by name.
---

# TypeScript patterns

Default assumption: a TypeScript project with `strict` mode enabled.

- If `strict` is off, that itself is a finding.
- Verify version-sensitive claims (which flags exist, which syntax is available) against the `typescript` version pinned in the project's `package.json` and lockfile before applying deltas below.

## Reference files

| File                                                             | Covers                                                                                                     |
| ---------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| [reference/strictness.md](reference/strictness.md)               | Strictness flags, tsconfig review, incremental migration patterns                                          |
| [reference/type-expressions.md](reference/type-expressions.md)   | `unknown` vs `any`, `satisfies`, discriminated unions, branded types, generic constraints, common patterns |
| [reference/narrowing.md](reference/narrowing.md)                 | `typeof` / `in` / `instanceof`, predicates, exhaustiveness, `assertNever`                                  |
| [reference/module-boundaries.md](reference/module-boundaries.md) | `import type`, project references, circular imports, barrel-file anti-pattern                              |
| [reference/anti-patterns.md](reference/anti-patterns.md)         | Twelve review-time anti-patterns with severity calls                                                       |

## References

- TypeScript handbook: https://www.typescriptlang.org/docs/handbook/
- tsconfig reference: https://www.typescriptlang.org/tsconfig
- Release notes index: https://www.typescriptlang.org/docs/handbook/release-notes/overview.html
- `satisfies` operator (4.9): https://devblogs.microsoft.com/typescript/announcing-typescript-4-9/
- Variance annotations (4.7): https://devblogs.microsoft.com/typescript/announcing-typescript-4-7/
- `verbatimModuleSyntax` and `const` type parameters (5.0): https://devblogs.microsoft.com/typescript/announcing-typescript-5-0/
- typescript-eslint rules (`no-empty-object-type`, `prefer-ts-expect-error`, `no-explicit-any`): https://typescript-eslint.io/rules/
- Vercel: package import optimization (barrel files): https://vercel.com/blog/how-we-optimized-package-imports-in-next-js

## Version notes

Checked: 2026-09-12 against https://devblogs.microsoft.com/typescript/ and Context7 /microsoft/typescript

- 7.0 (2026-07, latest 7.0.2): native Go compiler. Removed the options 6.0 deprecated: `module` none/amd/umd/system, `moduleResolution` node10/classic, `baseUrl`, `outFile`, `downlevelIteration`, `target: ES5`, `alwaysStrict: false`, `esModuleInterop: false`, `allowSyntheticDefaultImports: false`. A tsconfig using any of these does not compile on 7.
- 6.0: last JavaScript-based compiler; deprecated the options above.
- 5.0 (2023-03): `verbatimModuleSyntax`, `const` type parameters. 4.9: `satisfies`. 4.7: `in`/`out` variance annotations.
