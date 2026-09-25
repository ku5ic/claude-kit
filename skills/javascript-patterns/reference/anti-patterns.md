# Anti-patterns

Severity rubric:

- `failure`: a concrete defect or violation that should not ship.
- `warning`: a smell or pattern that compounds with other findings.
- `info`: a hardening opportunity or note, not a defect.

## Table of contents

- [`var` declarations](#var-declarations)
- [`==` and `!=`](#-and-)
- [Missing `await`](#missing-await)
- [Unhandled promise rejection](#unhandled-promise-rejection)
- [`eval()` and `Function()` constructor](#eval-and-function-constructor)
- [`console.log` left in production code](#consolelog-left-in-production-code)
- [Mutating function arguments](#mutating-function-arguments)
- [Implicit globals](#implicit-globals)
- [Float arithmetic for money](#float-arithmetic-for-money)
- [Mixing CJS and ESM in one file](#mixing-cjs-and-esm-in-one-file)
- [Missing or unchecked lockfile](#missing-or-unchecked-lockfile)
- [Missing `engines.node`](#missing-enginesnode)

## `var` declarations

`warning`. Function-scoped, hoisted with `undefined`, and re-declarable. In modern JS, `const` and `let` are correct in every place `var` would have been.

## `==` and `!=`

`warning`. Loose equality coerces operands and produces surprising results (`0 == ""`, `"0" == false`). Use `===` and `!==`. The single defensible exception is `x == null` for "null or undefined", but the explicit form is preferable.

## Missing `await`

`failure`. Calling an async function without `await` (and without a deliberate fire-and-forget pattern with `.catch()`) drops the return value, loses errors, and breaks ordering. Linters like `@typescript-eslint/no-floating-promises` catch this in TS projects; configure the equivalent in JS via `eslint-plugin-promise`.

## Unhandled promise rejection

`failure`. In current Node, an unhandled rejection is raised as an uncaught exception and the process exits. Every promise must be `await`ed, returned, or attached to a `.catch()`.

## `eval()` and `Function()` constructor

`failure`. Executes arbitrary string input as code. Cannot be safely used with any value derived from user input. There is almost always a structural alternative (lookup table, JSON parse, dispatch object).

## `console.log` left in production code

`warning`. Noise in production logs and a sign that proper instrumentation is missing. Use a structured logger (`pino`, `winston`, platform logger). Strip or guard the call before merging.

## Mutating function arguments

`warning`. A function that rewrites its inputs creates aliasing bugs at every call site that did not expect mutation. Either return a new value or document the mutation in the function name (`pushNewItem`, `mergeInto`).

## Implicit globals

`failure`. Assigning to an undeclared variable in non-strict mode silently creates a global. ES modules are strict by default; CJS files should start with `"use strict"` or run under `node --use-strict`. ESLint's `no-undef` catches this regardless of mode.

## Float arithmetic for money

`failure`. IEEE 754 doubles cannot represent most decimals exactly. Storing currency as a `Number` accumulates rounding error. Use integer minor units (cents, satoshis) or a decimal library.

## Mixing CJS and ESM in one file

`failure`. The two systems are not interchangeable inside a single module. Pick one per file. Cross-mode loading goes through dynamic `import()` (CJS to ESM) or `module.createRequire()` (ESM to CJS).

## Missing or unchecked lockfile

`warning`. Running `npm install`, `pnpm install`, etc. without a lockfile in the repo allows transitively-pulled versions to drift between environments. Always commit the lockfile that matches the package manager.

## Missing `engines.node`

`info`. Without an `engines.node` field, the package's supported Node range is implicit. Set it to the active LTS at minimum so consumers and CI fail loudly on incompatible runtimes.

## References

- Node.js process unhandledRejection: https://nodejs.org/api/process.html#event-unhandledrejection
- Node.js packages (engines, type): https://nodejs.org/api/packages.html
- MDN strict mode: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Strict_mode
