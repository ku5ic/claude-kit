# Async and promises

## `async`/`await` over `.then()`

`async` functions read like synchronous code with explicit await points and integrate cleanly with `try`/`catch`. Reach for `.then()` chains only when the operation is genuinely a stream of transformations and naming intermediate values would hurt readability. Mixed `await`/`then()` in the same function is harder to reason about than either style alone.

## `Promise.all`, `allSettled`, `race`, `any`

- `Promise.all(iter)` resolves to an array of resolved values; rejects on the first rejection (short-circuits).
- `Promise.allSettled(iter)` resolves to an array of result objects (`{ status: "fulfilled", value }` or `{ status: "rejected", reason }`); never rejects.
- `Promise.race(iter)` settles with whichever promise settles first (resolved or rejected).
- `Promise.any(iter)` resolves with the first promise to fulfill; rejects with `AggregateError` only when every input rejects.

Use `allSettled` when you need every result regardless of failures. Use `all` when one failure should abort the batch.

## `for await...of`

Iterate async iterables (and async generators):

```js
for await (const chunk of stream) {
  process(chunk);
}
```

Sequential by design. Use `Promise.all` over `Array.from(iter).map(fn)` when you actually want concurrency.

## Top-level `await`

ESM only. Inside a CJS module, top-level `await` is a syntax error. In ESM, the module evaluation pauses until the awaited value resolves; if it never resolves, Node exits with code 13.

## References

- MDN Promise: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Promise
- MDN async function: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Statements/async_function
- Node.js ESM (top-level await): https://nodejs.org/api/esm.html#top-level-await
