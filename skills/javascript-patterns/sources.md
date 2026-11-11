# javascript-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- MDN JavaScript reference: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference
- Node.js packages: https://nodejs.org/api/packages.html
- Node.js ESM: https://nodejs.org/api/esm.html
- Node.js process: https://nodejs.org/api/process.html
- Node.js previous releases (LTS schedule): https://nodejs.org/en/about/previous-releases
- TC39 proposals: https://github.com/tc39/proposals
- 2ality (Axel Rauschmayer): https://2ality.com/
- You Don't Know JS, 2nd ed. (Kyle Simpson): https://github.com/getify/You-Dont-Know-JS

### Version notes

Checked: 2026-11-10 against https://raw.githubusercontent.com/nodejs/Release/main/schedule.json, https://nodejs.org/en/blog/release/v24.0.0, https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html and https://github.com/tc39/proposals

- Node 26 "Lithium" Active LTS since 2026-10-28; Node 24 "Krypton" Maintenance LTS since 2026-10-20. Default assumption moved to 26. Corrected: `using` syntax ships in Node 24.0.0 (V8 13.6), and the `@satisfies` JSDoc tag is TS 5.0+ (the `satisfies` operator is 4.9).
- Explicit resource management (`using` / `await using`) reached Stage 4 in 2026; `reference/errors.md` updated, old `try`/`finally` advice kept as Legacy.

## reference/anti-patterns.md

### References

- Node.js process unhandledRejection: https://nodejs.org/api/process.html#event-unhandledrejection
- Node.js packages (engines, type): https://nodejs.org/api/packages.html
- MDN strict mode: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Strict_mode

## reference/async.md

### References

- MDN Promise: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Promise
- MDN async function: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Statements/async_function
- Node.js ESM (top-level await): https://nodejs.org/api/esm.html#top-level-await

## reference/collections.md

### References

- MDN Map: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Map
- MDN Set: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Set
- MDN WeakMap: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/WeakMap

## reference/equality-and-numbers.md

### References

- MDN equality comparisons: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Equality_comparisons_and_sameness
- MDN Number: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Number
- MDN BigInt: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/BigInt

## reference/errors.md

### References

- MDN Error: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Error
- MDN Error.cause: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Error/cause
- Node.js process unhandledRejection: https://nodejs.org/api/process.html#event-unhandledrejection
- TC39 explicit resource management: https://github.com/tc39/proposal-explicit-resource-management

## reference/jsdoc.md

### References

- TypeScript handbook, JS in TS: https://www.typescriptlang.org/docs/handbook/intro-to-js-ts.html
- JSDoc supported tags in TS: https://www.typescriptlang.org/docs/handbook/jsdoc-supported-types.html

## reference/modules.md

### References

- Node.js packages: https://nodejs.org/api/packages.html
- Node.js ESM: https://nodejs.org/api/esm.html

## reference/syntax.md

### References

- MDN JavaScript reference: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference
- Optional chaining: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Optional_chaining
- Nullish coalescing: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Nullish_coalescing
- Logical assignment: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Logical_AND_assignment
- `Array.prototype.at()`: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Array/at
- `Object.hasOwn()`: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Object/hasOwn
- Numeric separators: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Lexical_grammar#numeric_separators
