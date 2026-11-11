# typescript-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- TypeScript handbook: https://www.typescriptlang.org/docs/handbook/
- tsconfig reference: https://www.typescriptlang.org/tsconfig
- Release notes index: https://www.typescriptlang.org/docs/handbook/release-notes/overview.html
- `satisfies` operator (4.9): https://devblogs.microsoft.com/typescript/announcing-typescript-4-9/
- Variance annotations (4.7): https://devblogs.microsoft.com/typescript/announcing-typescript-4-7/
- `verbatimModuleSyntax` and `const` type parameters (5.0): https://devblogs.microsoft.com/typescript/announcing-typescript-5-0/
- typescript-eslint rules (`no-empty-object-type`, `ban-ts-comment`, `no-explicit-any`): https://typescript-eslint.io/rules/
- Vercel: package import optimization (barrel files): https://vercel.com/blog/how-we-optimized-package-imports-in-next-js

### Version notes

Checked: 2026-11-10 against https://devblogs.microsoft.com/typescript/announcing-typescript-6-0/, https://typescript-eslint.io/rules/prefer-ts-expect-error/ and the npm registry (7.0.2 latest; 7.1 in beta)

- 7.0 (2026-07, latest 7.0.2): native Go compiler. Removed the options 6.0 deprecated: `module` none/amd/umd/system, `moduleResolution` node10/classic, `baseUrl`, `outFile`, `downlevelIteration`, `target: ES5`, `alwaysStrict: false`, `esModuleInterop: false`, `allowSyntheticDefaultImports: false`. A tsconfig using any of these does not compile on 7.
- 6.0: last JavaScript-based compiler; deprecated the options above. `strict` now defaults to `true` and `types` to `[]`, so `@types/*` globals load only when listed.
- 5.0 (2023-03): `verbatimModuleSyntax`, `const` type parameters. 4.9: `satisfies`. 4.7: `in`/`out` variance annotations.

## reference/anti-patterns.md

### References

- typescript-eslint `no-explicit-any`: https://typescript-eslint.io/rules/no-explicit-any/
- typescript-eslint `no-empty-object-type`: https://typescript-eslint.io/rules/no-empty-object-type/
- typescript-eslint `ban-ts-comment` (replaces the deprecated `prefer-ts-expect-error`): https://typescript-eslint.io/rules/ban-ts-comment/
- TypeScript 3.9 release notes (`@ts-expect-error`): https://www.typescriptlang.org/docs/handbook/release-notes/typescript-3-9.html
- The Empty Object Type in TypeScript: https://www.totaltypescript.com/the-empty-object-type-in-typescript

## reference/module-boundaries.md

### References

- TypeScript handbook (modules): https://www.typescriptlang.org/docs/handbook/2/modules.html
- TypeScript handbook (project references): https://www.typescriptlang.org/docs/handbook/project-references.html
- Vercel: How we optimized package imports in Next.js (barrel-file analysis): https://vercel.com/blog/how-we-optimized-package-imports-in-next-js

### Claims

- Module boundaries, Project references for monorepos: `composite: true` per p...: https://www.typescriptlang.org/docs/handbook/project-references.html

## reference/narrowing.md

### References

- TypeScript handbook (narrowing): https://www.typescriptlang.org/docs/handbook/2/narrowing.html
- TypeScript handbook (discriminated unions): https://www.typescriptlang.org/docs/handbook/2/narrowing.html#discriminated-unions

## reference/strictness.md

### References

- tsconfig reference: https://www.typescriptlang.org/tsconfig
- TypeScript handbook (compiler options): https://www.typescriptlang.org/docs/handbook/compiler-options.html
- typescript-eslint shareable configs: https://typescript-eslint.io/users/configs/

## reference/type-expressions.md

### References

- TypeScript handbook (utility types): https://www.typescriptlang.org/docs/handbook/utility-types.html
- TypeScript handbook (template literal types): https://www.typescriptlang.org/docs/handbook/2/template-literal-types.html
- TypeScript 4.9 announcement (`satisfies`): https://devblogs.microsoft.com/typescript/announcing-typescript-4-9/
- TypeScript 5.0 announcement (`const` type parameters): https://devblogs.microsoft.com/typescript/announcing-typescript-5-0/
- TypeScript 4.7 announcement (variance annotations): https://devblogs.microsoft.com/typescript/announcing-typescript-4-7/
- Zod brand: https://zod.dev/
- Valibot brand action: https://valibot.dev/api/brand/
