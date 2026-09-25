# Anti-patterns to flag in review

- `any` on a public API surface without an escape-hatch comment: `failure`. Public types are contracts; `any` voids the contract silently.
- `as` cast where a type guard or `satisfies` would work: `warning`. Casts hide errors; predicates and `satisfies` surface them.
- `as unknown as X` to silence the compiler: `failure` unless a comment explains why no narrower path exists and what runtime check protects the boundary.
- Bare `as any` mid-expression: `failure`. At least `as unknown as X` announces the double escape; `as any` hides inside an expression and degrades downstream inference.
- `// @ts-nocheck` at the file head: `failure`. Disables typechecking for the whole file. Replace with targeted `// @ts-expect-error` lines or fix the underlying issue.
- `// @ts-ignore` over `// @ts-expect-error`: `warning`. `@ts-expect-error` (since 3.9) fails the build when the suppressed error goes away, forcing cleanup; `@ts-ignore` rots silently.
- Enum used in an API contract crossed with another team or service: `warning`. Discriminated unions or string-literal unions serialize cleanly; enums embed an integer-or-string ambiguity that breaks across runtimes.
- `Function`, `Object`, or `{}` as a parameter type: `failure`. `{}` means "any non-null, non-undefined value" (including `0`, `""`, primitives), NOT "an empty object". Use `Record<string, unknown>` for "any object", `unknown` for "any value", `Record<string, never>` for "actually empty".
- `namespace` keyword in `.ts` source: `warning`. Pre-modules era. Acceptable only in ambient declarations in `.d.ts`.
- Exported function with no explicit return type: `info`. Inference is fine internally; the public surface should pin the contract so callers do not silently track implementation drift.
- Tuple-shaped literal without `as const`: `info`. `[1, 2]` infers as `number[]`; `[1, 2] as const` infers as `readonly [1, 2]`. Choose deliberately.
- Excessive type acrobatics: a generic helper that takes more than a few minutes to read is debt. `warning`. Consider whether a simpler shape with a runtime helper would carry the same invariant.

## References

- typescript-eslint `no-explicit-any`: https://typescript-eslint.io/rules/no-explicit-any/
- typescript-eslint `no-empty-object-type`: https://typescript-eslint.io/rules/no-empty-object-type/
- typescript-eslint `prefer-ts-expect-error`: https://typescript-eslint.io/rules/prefer-ts-expect-error/
- TypeScript 3.9 release notes (`@ts-expect-error`): https://www.typescriptlang.org/docs/handbook/release-notes/typescript-3-9.html
- The Empty Object Type in TypeScript: https://www.totaltypescript.com/the-empty-object-type-in-typescript
