---
name: javascript-patterns
description: JavaScript patterns for modern syntax, async, ESM and CJS, and errors. Load before any edit to a .js, .mjs, .cjs, or .jsx file or package.json, however small, and for JavaScript questions.
---

# JavaScript patterns

Default assumption: a project running on the current Node.js Active LTS (Node 26, "Lithium") or a modern browser baseline.

- A project that also uses TypeScript: the type-aware skill applies on top.
- Match advice to the Node version in `.nvmrc`, `.tool-versions`, or `engines` in `package.json`.

## Reference files

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                                                          | Read                                                                   |
| ------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| Choosing ESM or CJS, or writing an import, an export, or a dynamic `import()`   | [reference/modules.md](reference/modules.md)                           |
| Writing modern syntax: `at()`, `Object.hasOwn`, `??=`, `?.`, or `??`            | [reference/syntax.md](reference/syntax.md)                             |
| Writing or changing async code: `await`, `Promise.*`, or `for await...of`       | [reference/async.md](reference/async.md)                               |
| Writing or changing error handling, an Error subclass, or a `using` declaration | [reference/errors.md](reference/errors.md)                             |
| Comparing values, checking for NaN or arrays, or doing arithmetic on money      | [reference/equality-and-numbers.md](reference/equality-and-numbers.md) |
| Choosing between a plain object, a `Map`, a `Set`, or a weak collection         | [reference/collections.md](reference/collections.md)                   |
| Adding JSDoc types or `// @ts-check`, or weighing a move to TypeScript          | [reference/jsdoc.md](reference/jsdoc.md)                               |
| Writing, changing, or reviewing JavaScript code                                 | [reference/anti-patterns.md](reference/anti-patterns.md)               |
