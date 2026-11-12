# Anti-patterns to flag in review

- `// @ts-ignore` where `// @ts-expect-error` (3.9+) would work: `warning`. Use `@ts-expect-error`.
- `any` on a public API surface without an escape-hatch comment: `failure`.
- `as unknown as X` to silence the compiler: `failure`, unless a comment says why no narrower path exists and which runtime check guards the boundary.
- Bare `as any` mid-expression: `failure`. At minimum, write `as unknown as X`.
- `// @ts-nocheck` at the file head: `failure`. Use targeted `// @ts-expect-error` lines, or fix the cause.
- `Function`, `Object`, or `{}` as a parameter type: `failure`. `{}` is any non-nullish value, primitives included. Use `Record<string, unknown>` for any object, `unknown` for any value, `Record<string, never>` for an empty object.
- An `as` cast where a type guard or `satisfies` would work: `warning`.
- An enum in an API contract shared with another team or service: `warning`. Use a discriminated union or a string-literal union.
- `namespace` in `.ts` source: `warning`. Only in ambient declarations in `.d.ts`.
- A generic helper that takes more than a few minutes to read: `warning`. Consider a simpler shape with a runtime helper that carries the same invariant.
- An exported function with no explicit return type: `info`.
- A tuple-shaped literal without `as const`: `info`. `[1, 2]` infers `number[]`, `[1, 2] as const` infers `readonly [1, 2]`. Choose deliberately.
