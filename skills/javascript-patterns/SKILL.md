---
name: javascript-patterns
description: JavaScript language patterns for modern syntax, async handling, ESM/CJS module systems, error handling, and review checklist, with or without TypeScript on top. Use whenever the project contains `.js`/`.mjs`/`.cjs`/`.jsx` files or a `package.json`, OR the user asks about JavaScript, its async model, its module system, or its package tooling, even if "JavaScript" is not mentioned by name.
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

Checked: 2026-11-10 against https://raw.githubusercontent.com/nodejs/Release/main/schedule.json, https://nodejs.org/en/blog/release/v24.0.0, https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html and https://github.com/tc39/proposals

- Node 26 "Lithium" Active LTS since 2026-10-28; Node 24 "Krypton" Maintenance LTS since 2026-10-20. Default assumption moved to 26. Corrected: `using` syntax ships in Node 24.0.0 (V8 13.6), and the `@satisfies` JSDoc tag is TS 5.0+ (the `satisfies` operator is 4.9).
- Explicit resource management (`using` / `await using`) reached Stage 4 in 2026; `reference/errors.md` updated, old `try`/`finally` advice kept as Legacy.
