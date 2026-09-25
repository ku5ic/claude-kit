# Errors

## Throw `Error` instances, not strings

`throw "bad input"` produces a value with no stack trace and bypasses `instanceof` checks. Always throw an `Error` (or a subclass). Future readers and runtime tooling rely on the `name`, `message`, `stack`, and `cause` properties.

## Use specific Error subclasses

The platform ships standard subclasses for common kinds of failure: `TypeError`, `RangeError`, `SyntaxError`, `URIError`, `ReferenceError`, plus `AggregateError` for batched failures. Throw the most specific one. Define your own subclass when callers will branch on the kind:

```js
class ValidationError extends Error {
  constructor(message, fields) {
    super(message);
    this.name = "ValidationError";
    this.fields = fields;
  }
}
```

## Narrow before reading custom properties

In `catch (err)`, `err` is typed `unknown` (in TypeScript) or arbitrary at runtime: `throw` accepts any value. Always check `instanceof X` before reading `.fields`, `.code`, etc.

```js
try {
  validate(input);
} catch (err) {
  if (err instanceof ValidationError) {
    return { fields: err.fields };
  }
  throw err;
}
```

## `Error.cause` (ES2022)

Wrap a lower-level error while preserving the original via the `cause` option:

```js
try {
  await connectToDatabase();
} catch (err) {
  throw new Error("Database connection failed", { cause: err });
}
```

The wrapper carries context for human readers; `err.cause` carries the original for tooling.

## Unhandled rejections terminate the process

In current Node.js, an unhandled promise rejection is raised as an uncaught exception by default and the process exits with a non-zero code. The `--unhandled-rejections` flag (default `throw`) controls this. Always either `await`, `return`, or `.catch()` every promise. Fire-and-forget needs an explicit `.catch(logAndContinue)` so the rejection does not propagate.

## Empty `catch {}`

`failure`. Swallowing an error silently hides defects. If the failure is genuinely expected and recoverable, log it (with the value) and continue, or re-throw a wrapped error explaining why it was caught.

## `using` and `await using`

Explicit resource management adds `using` (sync) and `await using` (async) declarations that auto-dispose at scope exit. It reached TC39 Stage 4 in 2026 and is part of the ECMAScript spec; Node ships it unflagged (Context7 /nodejs/node cites 20.4.0 as the floor; confirm against the project's Node version before relying on it). Prefer it over hand-written `try`/`finally` for anything with a `[Symbol.dispose]` or `[Symbol.asyncDispose]` method.

### Legacy (pre-Stage 4)

On a toolchain that does not yet parse `using`, keep `try`/`finally` for deterministic cleanup. The proposal was Stage 3 until 2026, so older transpiler configs may need a plugin.

## References

- MDN Error: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Error
- MDN Error.cause: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Error/cause
- Node.js process unhandledRejection: https://nodejs.org/api/process.html#event-unhandledrejection
- TC39 explicit resource management: https://github.com/tc39/proposal-explicit-resource-management
