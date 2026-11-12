# Anti-patterns

Severity levels: the report-format skill.

- An async call with no `await` and no deliberate fire-and-forget `.catch()`: `failure`. `@typescript-eslint/no-floating-promises` catches it in TS; in JS, configure `eslint-plugin-promise`.
- A promise neither awaited, returned, nor `.catch()`ed: `failure`. Current Node exits on an unhandled rejection.
- `eval()` or the `Function()` constructor: `failure`. Never on a value derived from user input; use a lookup table, `JSON.parse`, or a dispatch object.
- Float arithmetic for money: `failure`. Use integer minor units (cents, satoshis) or a decimal library.
- `require` and `import` in one file: `failure`. Pick one; bridge with `import()` (CJS to ESM) or `module.createRequire()` (ESM to CJS).
- `var`: `warning`. Use `const`, or `let` when it's reassigned.
- `==` or `!=`: `warning`. Use `===` and `!==`. `x == null` is the one defensible exception, and the explicit form is preferred.
- `console.log` left in production code: `warning`. Delete leftover debugging (`skills/kit-rules/reference/change.md` section 4); logging that stays goes through a structured logger (`pino`, `winston`, the platform's).
- A function that mutates its arguments: `warning`. Return a new value, or name the mutation (`pushNewItem`, `mergeInto`).
- No committed lockfile: `warning`. Commit the one that matches the package manager.
- No `engines.node`: `info`. Set it to the active LTS at minimum.
