---
name: typescript-patterns
description: TypeScript patterns for strictness, any vs unknown, satisfies, unions, generics, and module boundaries. Use for .ts or .tsx files, tsconfig*.json, or TypeScript questions.
---

# TypeScript patterns

Default assumption: a TypeScript project with `strict` mode enabled.

- From 6.0 `strict` defaults to `true`, so off means an explicit `strict: false`. Off is a finding.
- Check version-sensitive claims (which flags exist, which syntax is available) against the `typescript` version in the project's `package.json` and lockfile before applying a delta below.

## Reference files

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have.

| Before                                                                                                   | Read                                                             |
| -------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| Reviewing or changing a tsconfig, or turning on a compiler flag                                          | [reference/strictness.md](reference/strictness.md)               |
| Writing a type: a generic, a union, a branded type, `satisfies`, or choosing between `unknown` and `any` | [reference/type-expressions.md](reference/type-expressions.md)   |
| Narrowing a value with `typeof`, `in`, `instanceof`, a type predicate, or an exhaustive check            | [reference/narrowing.md](reference/narrowing.md)                 |
| Adding an import, a barrel file, or a project reference, or untangling a circular import                 | [reference/module-boundaries.md](reference/module-boundaries.md) |
| Reviewing TypeScript code                                                                                | [reference/anti-patterns.md](reference/anti-patterns.md)         |
