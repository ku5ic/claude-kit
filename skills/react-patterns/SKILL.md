---
name: react-patterns
description: React patterns for hooks rules, component design, and rendering performance. Load before any edit to a .jsx or .tsx file, however small, and for React questions.
---

# React patterns

Default assumption: React 19 with the modern (concurrent) renderer.

- Most rules hold unchanged on React 18. The 19-only APIs (`use`, `useActionState`, `useFormStatus`, `useOptimistic`, Actions) say so where they appear.
- Server Components are a feature of frameworks built on React; that framework's patterns load when its signals are in the project.
- Match advice to the React version in the project's `package.json` or lockfile.

## Reference files

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                                                | Read                                                     |
| --------------------------------------------------------------------- | -------------------------------------------------------- |
| Writing a hook, an effect, a list with keys, or conditional rendering | [reference/correctness.md](reference/correctness.md)     |
| Memoizing, optimizing a render, or adding context                     | [reference/performance.md](reference/performance.md)     |
| Handling a form or an action, or deciding where state lives           | [reference/structure.md](reference/structure.md)         |
| Writing, changing, or reviewing React code                            | [reference/anti-patterns.md](reference/anti-patterns.md) |
| Splitting a component, or deciding whether to                         | [reference/when-to-split.md](reference/when-to-split.md) |
