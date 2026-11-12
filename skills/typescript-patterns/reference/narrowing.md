# Narrowing

- A primitive: `typeof x === "string"`. It works for `string`, `number`, `boolean`, `bigint`, `symbol`, `undefined`, `function`, and `object`.
- Two object types with no shared discriminator: `"key" in obj`.
- A class-tagged value: `instanceof Foo`, mostly for DOM and Node built-ins.
- A check with real runtime work: a named type predicate, `function isFoo(x: unknown): x is Foo`, not an `as`. The compiler doesn't verify its body.
- A discriminated union: narrow with `if (msg.kind === "error")`. Keep the discriminator a string literal, never an enum.
- A `switch` over a union: end it with `default: return assertNever(msg)`, where `assertNever(x: never): never` throws. A new variant then fails the typecheck at `default`.
