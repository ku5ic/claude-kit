---
name: typescript-patterns
description: TypeScript patterns for strictness, any vs unknown, satisfies, unions, generics, and module boundaries. Use for .ts or .tsx files, tsconfig*.json, or TypeScript questions.
---

# TypeScript patterns

Default assumption: a TypeScript project with `strict` mode enabled.

- From 6.0 `strict` defaults to `true`, so off means an explicit `strict: false`. Off is a finding.
- Check version-sensitive claims (which flags exist, which syntax is available) against the `typescript` version in the project's `package.json` and lockfile before applying a delta below.

## Reference files

| File                                                             | Covers                                                                                                     |
| ---------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| [reference/strictness.md](reference/strictness.md)               | Flags by version, tsconfig review, turning flags on one at a time                                          |
| [reference/type-expressions.md](reference/type-expressions.md)   | `const` type parameters, `satisfies`, variance, `unknown` vs `any`, unions, branded types, common patterns |
| [reference/narrowing.md](reference/narrowing.md)                 | `typeof` / `in` / `instanceof`, predicates, `assertNever`                                                  |
| [reference/module-boundaries.md](reference/module-boundaries.md) | `import type`, project references, circular imports, barrel files                                          |
| [reference/anti-patterns.md](reference/anti-patterns.md)         | Twelve review-time anti-patterns with severity calls                                                       |
