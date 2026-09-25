# Equality and numbers

## `===` and `!==` over `==` and `!=`

Strict equality compares value and type without coercion. Loose equality (`==`) applies the abstract equality algorithm with surprising edge cases (`0 == ""`, `null == undefined`, `[] == false`). Use `===` and `!==` everywhere except the single intentional `x == null` idiom for "null or undefined" (and even then, prefer the more explicit `x === null || x === undefined`).

## `typeof null === "object"`

A historical quirk preserved for compatibility. To check for null, write `value === null`. `typeof` is correct for `"undefined"`, `"boolean"`, `"number"`, `"string"`, `"bigint"`, `"symbol"`, `"function"`, but not for distinguishing arrays or null from objects.

## `Number.isNaN` over global `isNaN`

The global `isNaN` coerces its argument first, so `isNaN("foo") === true`. `Number.isNaN` returns `true` only for the actual `NaN` value. Same idea for `Number.isFinite` over the global `isFinite`.

## `Array.isArray` over `instanceof Array`

`Array.isArray(value)` works across realms (e.g. iframes, Worker threads); `instanceof` does not because each realm has its own `Array` constructor.

## IEEE 754 traps

JavaScript numbers are 64-bit floats. The classics:

- `0.1 + 0.2 === 0.30000000000000004`. Never compare floats with `===`. Use a tolerance: `Math.abs(a - b) < Number.EPSILON`.
- `Number.MAX_SAFE_INTEGER === 2 ** 53 - 1`. Integers larger than this lose precision. Use `BigInt` for IDs from upstream systems that exceed 53 bits.
- `Math.round(0.5) === 1` but `Math.round(-0.5) === 0` (round-half-to-positive-infinity).

## Money is not a float

Storing currency as a `Number` is `failure`. Either store integer minor units (cents, satoshis) or use a decimal library (`decimal.js`, `dinero.js`). Float arithmetic on prices accumulates rounding error.

## `BigInt`

Arbitrary-precision integers. Literal suffix `n`: `1_000_000_000_000n`. Cannot mix `BigInt` and `Number` in arithmetic without explicit coercion. `JSON.stringify` does not serialize `BigInt`; use a replacer or convert to string at the boundary.

## References

- MDN equality comparisons: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Equality_comparisons_and_sameness
- MDN Number: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Number
- MDN BigInt: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/BigInt
