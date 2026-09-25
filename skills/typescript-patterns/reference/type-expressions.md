# Type expressions

## Contents

- [Core types](#core-types)
- [satisfies operator](#satisfies-operator)
- [Discriminated unions](#discriminated-unions)
- [Branded types](#branded-types)
- [Generic constraints](#generic-constraints)
- [Return types](#return-types)
- [Common patterns](#common-patterns)

## Core types

- `unknown` over `any`. `any` opts out of the type system; `unknown` keeps the burden of proof at the call site. `any` is allowed only as a documented escape hatch with `// eslint-disable-next-line @typescript-eslint/no-explicit-any` plus a one-line reason comment.
- `as` casts: a smell unless paired with a type guard (`x is Foo`) or `satisfies`. `as unknown as X` chains earn a `failure` unless commented; the double escape hatch defeats the type system.

## satisfies operator

`satisfies` (since 4.9) vs annotation: `satisfies` validates compatibility while preserving the inferred narrow type; an annotation widens. Use `satisfies` when you want the literal/inferred specificity to survive; use an annotation when widening is the point.

```ts
// annotation widens: routes.home is `string`
const routes: Record<string, string> = {
  home: "/",
  signin: "/auth/signin",
};

// satisfies preserves: routes.home is the literal "/"
const routes = {
  home: "/",
  signin: "/auth/signin",
} satisfies Record<string, string>;
```

`satisfies` plus `as const`: `as const` freezes literal inference (no widening to `string`, arrays become readonly tuples); `satisfies` checks the frozen value against a contract without widening it back. The combination produces stronger inference than either alone:

```ts
const routes = {
  home: "/",
  user: "/users/:id",
} as const satisfies Record<string, `/${string}`>;
// routes.user is the literal "/users/:id", and the contract is enforced.
```

Use the pair when the value has a runtime shape worth preserving (route tables, permission maps, config objects with literal-typed values).

## Discriminated unions

- Discriminated unions over enums for shared API contracts. The discriminator is a literal `kind` (or `type`) field; `switch` over it gives free exhaustiveness. Enums are fine for closed sets inside a single module; they cross service or language boundaries badly.
- Discriminator naming: `kind` and `type` are both common; pick one and stay consistent within a codebase. `type` collides with the TypeScript keyword in declaration positions and with framework-domain "type" fields (event types, action types); `kind` is unambiguous in those contexts.
- Serialization: a discriminated union serializes cleanly because the discriminator is a literal string. Enums embed an integer-or-string ambiguity at the JSON boundary; consumers in other languages need to know which form ships. String-literal unions avoid that.
- Exhaustiveness via `assertNever`: pair the union switch with a `default` branch that asserts unreachability. Adding a new variant becomes a typecheck error, not a runtime surprise. Pattern detail lives in `narrowing.md`.

## Branded types

Branded types for IDs and money: zero runtime cost, structural mismatch enough to keep them apart.

```ts
type UserId = string & { readonly __brand: "UserId" };
type OrderId = string & { readonly __brand: "OrderId" };

function asUserId(s: string): UserId {
  // validate s here (length, prefix, schema)
  return s as UserId;
}
```

The brand exists only at the type level. Construct branded values at boundaries (parsers, decoders, schema validators), not throughout the codebase.

Composing brands: brands compose with intersection. A brand that carries multiple invariants (verified email, positive integer) is a chain of phantom properties.

```ts
type Email = string & { readonly __email: true };
type Verified = { readonly __verified: true };
type VerifiedEmail = Email & Verified;
```

Each phantom property is a separate compile-time witness; the runtime value is still a plain string.

Parser-pattern integration: when the value enters from outside (HTTP body, file, queue message), let the validator brand it on success. Both Zod and Valibot expose first-class brand support.

```ts
// Zod
const UserId = z.string().uuid().brand<"UserId">();
type UserId = z.infer<typeof UserId>;

// Valibot
const UserId = v.pipe(v.string(), v.uuid(), v.brand("UserId"));
type UserId = v.InferOutput<typeof UserId>;
```

The validator's success path is the only way to construct the branded value; downstream code that takes a `UserId` parameter is guaranteed the value passed validation.

## Generic constraints

- Generic constraints: `<T extends Record<string, unknown>>` over bare `<T>` when the helper indexes into `T`. Bare generics that are immediately indexed produce confusing errors at the call site.
- `const` type parameters (since 5.0): `<const T>` lets a generic infer the literal/readonly form of an argument without forcing every caller to write `as const`. Use when the generic is meant to capture the exact shape of a passed-in literal (route tables, schema definitions).

```ts
function defineRoutes<const T extends Record<string, string>>(routes: T): T {
  return routes;
}
const r = defineRoutes({ home: "/", user: "/users/:id" });
// r.home is the literal "/", not `string`.
```

- Variance annotations `in` / `out` (since 4.7): explicit covariance (`out`) and contravariance (`in`) on a type parameter. Reach for these only when a complex generic chain is producing variance errors that the inference pass cannot solve. Most codebases never need them; when they do, the compiler tells you.

## Return types

- Return types on internal functions: inference is fine. On exported / public-API functions: explicit return types stop unintended widening as the implementation drifts.

## Common patterns

- Built-in utility types over hand-rolled: `Pick`, `Omit`, `Partial`, `Required`, `Readonly`. Hand-rolled equivalents drift.
- Extract types from values, not the other way around: `ReturnType<typeof fn>`, `Parameters<typeof fn>`, `Awaited<T>`. Reduces duplication at module boundaries; brittle when the source signature changes (the failure mode is loud, which is the point).
- Mapped types with `as` clauses to rename keys: `{ [K in keyof T as \`on${Capitalize<K & string>}\`]: () => void }`. Useful for deriving event-handler shapes from a state type.
- Template literal types for string-shape constraints: `type Pixels = \`${number}px\``. Compile-time only; pair with a runtime validator if the value crosses a network or storage boundary.
- Conditional types: useful but slow to compile and hard to read. Reach for them only when overloads or plain unions cannot do the job.
- `infer` in conditional types: extract a type from a generic shape (return type, parameter type, awaited type). Useful and dense; document the intent.

```ts
type Awaited<T> = T extends Promise<infer U> ? U : T;
```

## References

- TypeScript handbook (utility types): https://www.typescriptlang.org/docs/handbook/utility-types.html
- TypeScript handbook (template literal types): https://www.typescriptlang.org/docs/handbook/2/template-literal-types.html
- TypeScript 4.9 announcement (`satisfies`): https://devblogs.microsoft.com/typescript/announcing-typescript-4-9/
- TypeScript 5.0 announcement (`const` type parameters): https://devblogs.microsoft.com/typescript/announcing-typescript-5-0/
- TypeScript 4.7 announcement (variance annotations): https://devblogs.microsoft.com/typescript/announcing-typescript-4-7/
- Zod brand: https://zod.dev/
- Valibot brand action: https://valibot.dev/api/brand/
