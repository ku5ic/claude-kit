# Type expressions

- A generic meant to capture a passed-in literal's exact shape, on 5.0+: a `const` type parameter, `function defineRoutes<const T extends Record<string, string>>(routes: T): T`, so callers skip `as const`.
- Checking a value against a type on 4.9+: `satisfies` keeps the inferred literal type, an annotation widens it. Annotate only when widening is the point.
- A value whose literal shape matters (route tables, permission maps, literal-typed config): ``as const satisfies Record<string, `/${string}`>`` freezes the literals and enforces the contract.
- Variance annotations `in`/`out` on 4.7+: only when a generic chain produces variance errors that inference can't solve.
- A value of unknown type: `unknown`, never `any`. `any` only as a documented escape hatch, with `// eslint-disable-next-line @typescript-eslint/no-explicit-any` and a one-line reason.
- An `as` cast: pair it with a type guard (`x is Foo`) or use `satisfies`. An uncommented `as unknown as X` is a `failure`.

## Discriminated unions

- A shared API contract: a union discriminated by a literal `kind` or `type` field, or a string-literal union, never an enum. Enums are fine for closed sets inside one module.
- The discriminator name: `kind` or `type`, the same across the codebase. Where `type` collides with the keyword or a domain "type" field (event types, action types), `kind`.
- A `switch` over the union: end it with `assertNever` (`narrowing.md`).

## Branded types

- IDs and money: brand them, as in `type UserId = string & { readonly __brand: "UserId" }`.
- Construct branded values only at boundaries: parsers, decoders, schema validators.
- Several invariants (verified email, positive integer): intersect brands, `type VerifiedEmail = Email & Verified`, one phantom property each.
- A value from outside (HTTP body, file, queue message): let the validator brand it on success, Zod `z.string().uuid().brand<"UserId">()` or Valibot `v.pipe(v.string(), v.uuid(), v.brand("UserId"))`.

## Return types

- An exported function: give it an explicit return type. Internal functions can infer theirs.

## Common patterns

- Built-in utility types (`Pick`, `Omit`, `Partial`, `Required`, `Readonly`) over hand-rolled ones.
- Types from values, not the reverse: `ReturnType<typeof fn>`, `Parameters<typeof fn>`, `Awaited<T>`.
- Renaming keys: a mapped type with an `as` clause, ``{ [K in keyof T as `on${Capitalize<K & string>}`]: () => void }``.
- A string-shape constraint: a template literal type, `` type Pixels = `${number}px` ``. When the value crosses a network or storage boundary, add a runtime validator.
- Conditional types: only when overloads or plain unions can't do the job.
- `infer` in a conditional type, as in `T extends Promise<infer U> ? U : T`: document the intent.
