# JSDoc and `@ts-check` for type-checked JS

- `@satisfies {Type}` (TS 5.0+): check a value against a type without widening it.
- Types in a `.js` file: `// @ts-check` at the top plus JSDoc tags. Without it, type errors in `.js` are silent; with it they show in the editor and in `tsc --noEmit --allowJs --checkJs`.
- Tags: `@param {Type} name`, `@returns {Type}`, `@type {Type}` (`/** @type {string[]} */ ([])`), `@typedef {{ id: string, name: string }} User`, and `@template T` for generics.
- The whole project: `jsconfig.json` with `"checkJs": true`, plus `"strict": true` for a TypeScript project's strictness floor.
- Small JS-only codebases and libraries with no build step: JSDoc plus `@ts-check`. When type definitions reach the size of a small module, or generic helpers keep multiplying, move to TypeScript.
