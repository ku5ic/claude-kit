# Modern syntax

Current Node Active LTS runs all of these natively.

- ES2022, indexing from the end: `array.at(-1)`, not `array[array.length - 1]`.
- ES2022, an own-property check: `Object.hasOwn(obj, key)`, not `Object.prototype.hasOwnProperty.call(obj, key)`. It also works on `Object.create(null)` objects and on objects that override `hasOwnProperty`.
- ES2021, logical assignment: `a ??= b` assigns only when `a` is null or undefined, `||=` only when falsy, `&&=` only when truthy. Use one only when the condition is obvious; otherwise write an `if`.
- ES2021, numeric separators: `1_000_000`. One underscore at a time, never at the start or end, never after a leading `0`.
- ES2020, optional traversal: `obj?.prop`, `obj?.[key]`, `fn?.()` short-circuit to `undefined` on null or undefined. Never use it to skip a null check on a value that must be present.
- ES2020, a fallback only for missing values: `a ?? b`. It keeps `0`, `""`, and `false`, which `||` replaces.
- A binding: `const` first, `let` only when it's reassigned. Never `var`.
- Use a template literal for any string with interpolation or several lines.
- Use arrow functions, which have no own `this` or `arguments`, for callbacks, short composables, and code that needs lexical `this`. Use named declarations for top-level exports and recursive functions.
- Destructuring defaults fire only on `undefined`, never on other falsy values. Rename with `{ id: userId }`.
- Object spread is shallow, so nested objects keep sharing references with the source.
- A property named after its variable: `{ port }`. A method: `{ onClick() {} }`.
