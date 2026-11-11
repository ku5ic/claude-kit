# Equality and numbers

- Comparing: `===` and `!==`, never `==` or `!=`. The one exception is `x == null` for null or undefined, and `x === null || x === undefined` is preferred even there.
- A null check: `value === null`. `typeof null` is `"object"`, so `typeof` can't tell null or arrays from objects.
- A NaN check: `Number.isNaN`, not global `isNaN`, which coerces first (`isNaN("foo")` is `true`). Likewise `Number.isFinite` over `isFinite`.
- An array check: `Array.isArray`, which works across realms (iframes, Worker threads). Never `instanceof Array`.
- Comparing floats: within a tolerance, `Math.abs(a - b) < Number.EPSILON`, never `===`. `0.1 + 0.2` is `0.30000000000000004`.
- Integers past `Number.MAX_SAFE_INTEGER` (`2 ** 53 - 1`) lose precision. Upstream IDs over 53 bits: `BigInt`.
- `Math.round` rounds half toward positive infinity: `Math.round(0.5)` is `1`, `Math.round(-0.5)` is `0`.
- Money: integer minor units (cents, satoshis) or a decimal library (`decimal.js`, `dinero.js`). Currency stored as a `Number` is `failure`.
- `BigInt`: literal suffix `n` (`1_000_000_000_000n`). Coerce explicitly before mixing it with `Number` in arithmetic. `JSON.stringify` can't serialize it; use a replacer or convert to a string at the boundary.
