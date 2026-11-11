---
name: javascript-patterns
description: JavaScript patterns for modern syntax, async, ESM and CJS, and errors. Use for .js, .mjs, .cjs, or .jsx files, package.json, or JavaScript questions.
---

# JavaScript patterns

Default assumption: a project running on the current Node.js Active LTS (Node 26, "Lithium") or a modern browser baseline.

- If the project also uses TypeScript, the type-aware skill applies on top of these language patterns.
- Adapt advice to the Node version in the project's `.nvmrc`, `.tool-versions`, or `engines` field in `package.json`.

## Reference files

| File                                                                   | Covers                                                                           |
| ---------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| [reference/modules.md](reference/modules.md)                           | ESM and CJS, `package.json` `"type"`, dynamic `import()`, dual-package hazard    |
| [reference/syntax.md](reference/syntax.md)                             | Modern syntax: optional chaining, nullish coalescing, logical assignment, `at()` |
| [reference/async.md](reference/async.md)                               | `async`/`await`, `Promise.*`, `for await...of`, top-level `await`                |
| [reference/errors.md](reference/errors.md)                             | Error subclasses, `Error.cause`, unhandled rejection, `using` (advisory)         |
| [reference/equality-and-numbers.md](reference/equality-and-numbers.md) | `===`, `Number.isNaN`, `Array.isArray`, IEEE 754, `BigInt`                       |
| [reference/collections.md](reference/collections.md)                   | `Map`, `Set`, `WeakMap`, `WeakSet` and when to reach for each                    |
| [reference/jsdoc.md](reference/jsdoc.md)                               | `// @ts-check` plus JSDoc tags for editor-checked types in `.js` files           |
| [reference/anti-patterns.md](reference/anti-patterns.md)               | Twelve review-time anti-patterns with severity calls                             |
