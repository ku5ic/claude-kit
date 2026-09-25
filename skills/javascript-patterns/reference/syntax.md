# Modern syntax

Each entry lists the ECMAScript edition and the recommendation. Verified against MDN. Modern Node (current Active LTS) supports all of these natively.

## Table of contents

- [`const` / `let` over `var`](#const--let-over-var)
- [Template literals](#template-literals)
- [Arrow vs named functions](#arrow-vs-named-functions)
- [Destructuring](#destructuring)
- [Spread and rest](#spread-and-rest)
- [Optional chaining `?.` (ES2020)](#optional-chaining--es2020)
- [Nullish coalescing `??` (ES2020)](#nullish-coalescing--es2020)
- [Logical assignment `||=` `&&=` `??=` (ES2021)](#logical-assignment---es2021)
- [Object shorthand](#object-shorthand)
- [`Array.prototype.at()` (ES2022)](#arrayprototypeat-es2022)
- [`Object.hasOwn()` over `hasOwnProperty.call` (ES2022)](#objecthasown-over-hasownpropertycall-es2022)
- [Numeric separators (ES2021)](#numeric-separators-es2021)

## `const` / `let` over `var`

`var` is function-scoped, hoists with `undefined`, and is re-declarable. `const` and `let` are block-scoped and respect the temporal dead zone. Reach for `const` first; switch to `let` only when reassignment is real.

## Template literals

Backtick-delimited strings support `${expression}` interpolation and unescaped newlines. Use them for any string with interpolation or multi-line content. They also enable tagged templates, which are how libraries like `styled-components` and `gql` work.

## Arrow vs named functions

Arrow functions inherit `this` from the surrounding scope and have no `arguments` binding.

- Use arrow functions for callbacks, short composables, and any place where lexical `this` matters.
- Use named function declarations for top-level module exports and recursive functions, where the binding name appears in stack traces and helps debuggability.

## Destructuring

Pull fields from objects and arrays at the binding site:

```js
const { id, name = "anon" } = user;
const [first, ...rest] = items;
```

Default values activate only when the source field is `undefined`, not for any falsy value. Renaming uses `{ id: userId }`.

## Spread and rest

`...` spreads in expressions and collects in parameters / destructuring patterns:

```js
const merged = { ...base, ...overrides };
function tag(label, ...rest) {}
```

Object spread is shallow. Nested objects share references with the source.

## Optional chaining `?.` (ES2020)

`obj?.prop`, `obj?.[key]`, and `fn?.()` short-circuit to `undefined` when the left side is `null` or `undefined`, instead of throwing. Use it for traversal of optional shapes; do not use it to suppress real null checks where the value should always be present.

## Nullish coalescing `??` (ES2020)

`a ?? b` returns `b` only when `a` is `null` or `undefined`. Unlike `||`, it preserves `0`, `""`, and `false`:

```js
const port = config.port ?? 3000; // keeps 0 if explicitly set to 0
const port2 = config.port || 3000; // overrides 0
```

Use `??` whenever the fallback should fire only on missing values.

## Logical assignment `||=` `&&=` `??=` (ES2021)

```js
options.timeout ??= 30_000; // assign only if currently null/undefined
flags.debug ||= isDev; // assign only if currently falsy
cache.value &&= transform(cache.value); // assign only if currently truthy
```

Concise, but only reach for them when the conditional shape is obvious. Otherwise prefer an explicit `if`.

## Object shorthand

```js
const port = 3000;
const config = { port }; // == { port: port }
const handlers = {
  onClick() {
    /* ... */
  },
};
```

## `Array.prototype.at()` (ES2022)

Indexed access that supports negative indices: `array.at(-1)` returns the last element. Cleaner than `array[array.length - 1]`.

## `Object.hasOwn()` over `hasOwnProperty.call` (ES2022)

```js
if (Object.hasOwn(obj, "key")) {
  /* ... */
}
```

`Object.hasOwn(obj, key)` works for null-prototype objects (`Object.create(null)`) and for objects that overrode `hasOwnProperty`. MDN recommends it over `Object.prototype.hasOwnProperty.call(obj, key)`.

## Numeric separators (ES2021)

Underscores improve readability of numeric literals. They are stripped at parse time:

```js
const million = 1_000_000;
const bits = 0b1010_0001_1000_0101;
```

No more than one underscore in a row, not at the start or end, not after a leading `0`.

## References

- MDN JavaScript reference: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference
- Optional chaining: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Optional_chaining
- Nullish coalescing: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Nullish_coalescing
- Logical assignment: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Logical_AND_assignment
- `Array.prototype.at()`: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Array/at
- `Object.hasOwn()`: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Object/hasOwn
- Numeric separators: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Lexical_grammar#numeric_separators
