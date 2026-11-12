# Async and promises

- Async code: `async`/`await`. A `.then()` chain only for a real stream of transformations where naming each value would hurt. Don't mix both in one function.
- One failure should abort the batch: `Promise.all`, which rejects on the first rejection. Every result regardless of failures: `Promise.allSettled`, which never rejects and returns `{ status: "fulfilled", value }` or `{ status: "rejected", reason }`.
- `Promise.race` settles with the first promise to settle, fulfilled or rejected. `Promise.any` fulfills with the first to fulfill and rejects with `AggregateError` only when every input rejects.
- Async iterables and generators: `for await (const chunk of stream)`. It runs sequentially; for concurrency, `Promise.all` over `Array.from(iter).map(fn)`.
- Top-level `await`: ESM only, a syntax error in CJS. If it never resolves, Node exits with code 13.
