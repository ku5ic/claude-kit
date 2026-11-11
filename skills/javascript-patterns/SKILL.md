---
name: javascript-patterns
description: JavaScript patterns for modern syntax, async, ESM and CJS, and errors. Use for .js, .mjs, .cjs, or .jsx files, package.json, or JavaScript questions.
---

# JavaScript patterns

Default assumption: a project running on the current Node.js Active LTS (Node 26, "Lithium") or a modern browser baseline.

- A project that also uses TypeScript: the type-aware skill applies on top.
- Match advice to the Node version in `.nvmrc`, `.tool-versions`, or `engines` in `package.json`.

## Reference files

| File                                                                   | Covers                                                                        |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| [reference/modules.md](reference/modules.md)                           | ESM and CJS, `"type"`, `.mjs`/`.cjs`, dynamic `import()`, dual-package hazard |
| [reference/syntax.md](reference/syntax.md)                             | `at()`, `Object.hasOwn`, `??=`, `?.`, `??` by edition, then core syntax       |
| [reference/async.md](reference/async.md)                               | `async`/`await`, `Promise.*`, `for await...of`, top-level `await`             |
| [reference/errors.md](reference/errors.md)                             | `using`, `Error.cause`, Error subclasses, unhandled rejection                 |
| [reference/equality-and-numbers.md](reference/equality-and-numbers.md) | `===`, `Number.isNaN`, `Array.isArray`, IEEE 754, money, `BigInt`             |
| [reference/collections.md](reference/collections.md)                   | `Map`, `Set`, `WeakMap`, `WeakSet` and when to reach for each                 |
| [reference/jsdoc.md](reference/jsdoc.md)                               | `@satisfies`, `// @ts-check`, JSDoc tags, when to move to TypeScript          |
| [reference/anti-patterns.md](reference/anti-patterns.md)               | Eleven review-time anti-patterns with severity calls                          |
