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

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have.

| Before                                                                | Read                                                     |
| --------------------------------------------------------------------- | -------------------------------------------------------- |
| Writing a hook, an effect, a list with keys, or conditional rendering | [reference/correctness.md](reference/correctness.md)     |
| Memoizing, optimizing a render, or adding context                     | [reference/performance.md](reference/performance.md)     |
| Handling a form or an action, or deciding where state lives           | [reference/structure.md](reference/structure.md)         |
| Reviewing React code                                                  | [reference/anti-patterns.md](reference/anti-patterns.md) |
| Splitting a component, or deciding whether to                         | [reference/when-to-split.md](reference/when-to-split.md) |
