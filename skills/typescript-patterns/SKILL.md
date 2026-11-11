---
name: typescript-patterns
description: TypeScript patterns for strictness, any vs unknown, satisfies, unions, generics, and module boundaries. Use for .ts or .tsx files, tsconfig*.json, or TypeScript questions.
---

# TypeScript patterns

Default assumption: a TypeScript project with `strict` mode enabled.

- If `strict` is off, that itself is a finding. From 6.0 `strict` defaults to `true`, so off means an explicit `strict: false`.
- Verify version-sensitive claims (which flags exist, which syntax is available) against the `typescript` version pinned in the project's `package.json` and lockfile before applying deltas below.

## Reference files

| File                                                             | Covers                                                                                                     |
| ---------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| [reference/strictness.md](reference/strictness.md)               | Strictness flags, tsconfig review, incremental migration patterns                                          |
| [reference/type-expressions.md](reference/type-expressions.md)   | `unknown` vs `any`, `satisfies`, discriminated unions, branded types, generic constraints, common patterns |
| [reference/narrowing.md](reference/narrowing.md)                 | `typeof` / `in` / `instanceof`, predicates, exhaustiveness, `assertNever`                                  |
| [reference/module-boundaries.md](reference/module-boundaries.md) | `import type`, project references, circular imports, barrel-file anti-pattern                              |
| [reference/anti-patterns.md](reference/anti-patterns.md)         | Twelve review-time anti-patterns with severity calls                                                       |
