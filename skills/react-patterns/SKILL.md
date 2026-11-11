---
name: react-patterns
description: React patterns for hooks rules, component design, and rendering performance. Use for .jsx or .tsx files, react in package.json, or React questions.
---

# React patterns

Default assumption: React 19 with the modern (concurrent) renderer.

- Most rules apply unchanged to React 18; deltas (`use`, `useActionState`, `useFormStatus`, `useOptimistic`, Actions) are called out where they matter.
- Server Components are a feature of frameworks built on React; framework-specific patterns load when those framework signals are present in the project.
- Adapt advice to the version in the project's `package.json` or lockfile.

## Reference files

| File                                                     | Covers                                                                              |
| -------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| [reference/correctness.md](reference/correctness.md)     | Rules of Hooks, dependency arrays, key stability, effect cleanup, render pitfalls   |
| [reference/performance.md](reference/performance.md)     | Memoization costs, `React.memo`, virtualization, lazy init, context, React Compiler |
| [reference/structure.md](reference/structure.md)         | Controlled vs uncontrolled, derived state, prop drilling, effects vs handlers       |
| [reference/anti-patterns.md](reference/anti-patterns.md) | Seven review-time anti-patterns with severity calls                                 |
| [reference/when-to-split.md](reference/when-to-split.md) | Heuristics for component splitting and what does not justify a split                |
