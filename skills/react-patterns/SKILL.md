---
name: react-patterns
description: React patterns for hooks rules, component design, and rendering performance. Use for .jsx or .tsx files, react in package.json, or React questions.
---

# React patterns

Default assumption: React 19 with the modern (concurrent) renderer.

- Most rules hold unchanged on React 18. The 19-only APIs (`use`, `useActionState`, `useFormStatus`, `useOptimistic`, Actions) say so where they appear.
- Server Components are a feature of frameworks built on React; that framework's patterns load when its signals are in the project.
- Match advice to the React version in the project's `package.json` or lockfile.

## Reference files

| File                                                     | Covers                                                                                  |
| -------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| [reference/correctness.md](reference/correctness.md)     | `use()`, Rules of Hooks, dependency arrays, keys, effect cleanup, conditional rendering |
| [reference/performance.md](reference/performance.md)     | `useOptimistic`, memoization, React Compiler, virtualization, lazy init, context        |
| [reference/structure.md](reference/structure.md)         | Actions, controlled vs uncontrolled, derived state, prop drilling, effects vs handlers  |
| [reference/anti-patterns.md](reference/anti-patterns.md) | Seven review-time anti-patterns with severity calls                                     |
| [reference/when-to-split.md](reference/when-to-split.md) | When to split a component, and what doesn't justify a split                             |
