# Narrowing

- `typeof x === "string"` for primitive narrowing. Works for `string`, `number`, `boolean`, `bigint`, `symbol`, `undefined`, `function`, `object`.
- `"key" in obj` for object-shape narrowing without committing to a class hierarchy. The cleanest way to distinguish two object types that do not share a discriminator.
- `instanceof Foo` for class-tagged values. Rare in modern TS outside DOM and Node built-ins; class hierarchies are not where most narrowing happens.
- User-defined type predicates: `function isFoo(x: unknown): x is Foo { ... }`. Use over `as` when the check has real runtime work that deserves a name. The body of the predicate is unverified by the compiler; a buggy predicate lies to the type system.
- Discriminator narrowing on unions: `if (msg.kind === "error") { /* msg.error is now in scope */ }`. Keep the discriminator a string literal (`"error" | "ok"`), not an enum.
- Exhaustiveness check: end a `switch` over a union with `default: { const _exhaustive: never = value; throw new Error("unhandled"); }`. Adding a new variant becomes a typecheck error, not a runtime surprise.

## assertNever helper

The exhaustiveness pattern reads better as a named helper. The body is the same `never` assignment, but the call site is one line:

```ts
function assertNever(x: never): never {
  throw new Error(`unhandled variant: ${JSON.stringify(x)}`);
}

switch (msg.kind) {
  case "ok":
    return handleOk(msg);
  case "error":
    return handleError(msg);
  default:
    return assertNever(msg);
}
```

The compiler enforces that `assertNever`'s argument has type `never`, which is only true if the switch is exhaustive. Adding a new union variant breaks the typecheck at the `default` branch, exactly where the missing case needs to be handled.

## References

- TypeScript handbook (narrowing): https://www.typescriptlang.org/docs/handbook/2/narrowing.html
- TypeScript handbook (discriminated unions): https://www.typescriptlang.org/docs/handbook/2/narrowing.html#discriminated-unions
