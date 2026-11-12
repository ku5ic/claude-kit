# Errors

- Cleanup on Node 24.0.0+ (V8 13.6): `using` and `await using` over hand-written `try`/`finally` for anything with `[Symbol.dispose]` or `[Symbol.asyncDispose]`. Node 20.4.0 only added `filehandle[Symbol.asyncDispose]`.
- A toolchain that can't parse `using` yet: keep `try`/`finally`. Older transpiler configs may need a plugin.
- ES2022, wrapping a lower-level error: `throw new Error("Database connection failed", { cause: err })`.
- Throwing: an `Error` or a subclass, never a string.
- A failure kind: the most specific built-in (`TypeError`, `RangeError`, `SyntaxError`, `URIError`, `ReferenceError`, `AggregateError` for batches). When callers branch on the kind, define your own subclass that sets `this.name`.
- In `catch (err)`: check `err instanceof X` before reading `.fields`, `.code`, or any custom property. `throw` accepts any value.
- Every promise: `await` it, return it, or `.catch()` it. An unhandled rejection exits the process (`--unhandled-rejections` defaults to `throw`). Fire-and-forget needs an explicit `.catch(logAndContinue)`.
