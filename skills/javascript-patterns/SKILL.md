---
name: javascript-patterns
description: JavaScript language patterns - modern syntax, async handling, ESM/CJS module systems, error handling, and review checklist, for JavaScript without TypeScript. Use whenever the project contains `.js`/`.mjs`/`.cjs`/`.jsx` files and no `tsconfig.json`, OR the user asks about JavaScript, its async model, its module system, or its package tooling, even if "JavaScript" is not mentioned by name.
---

# JavaScript patterns

Default assumption: a project running on the current Node.js Active LTS (Node 24, "Krypton") or a modern browser baseline.

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

## References

- MDN JavaScript reference: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference
- Node.js packages: https://nodejs.org/api/packages.html
- Node.js ESM: https://nodejs.org/api/esm.html
- Node.js process: https://nodejs.org/api/process.html
- Node.js previous releases (LTS schedule): https://nodejs.org/en/about/previous-releases
- TC39 proposals: https://github.com/tc39/proposals
- 2ality (Axel Rauschmayer): https://2ality.com/
- You Don't Know JS, 2nd ed. (Kyle Simpson): https://github.com/getify/You-Dont-Know-JS

## Version notes

Checked: 2026-09-12 against https://nodejs.org/en/about/previous-releases and https://github.com/tc39/proposals

- Node 26 is Current (2026-05); Node 24 "Krypton" is Active LTS (2025-05). Default assumption above stays on the LTS.
- Explicit resource management (`using` / `await using`) reached Stage 4 in 2026; `reference/errors.md` updated, old `try`/`finally` advice kept as Legacy.
- `@satisfies` JSDoc tag: TS 4.9+, unchanged.
