# JSDoc and `@ts-check` for type-checked JS

JavaScript without TypeScript can still get editor-checked types. Add `// @ts-check` at the top of the file and use JSDoc tags for the shapes the inferencer cannot pick up. The TypeScript compiler treats annotated `.js` the same way it treats `.ts` for type checking.

## Enable per file

```js
// @ts-check

/**
 * @param {string} email
 * @param {{ requireMx?: boolean }} [options]
 * @returns {boolean}
 */
function isValidEmail(email, options) {
  // ...
}
```

Without `// @ts-check`, type errors in a `.js` file are silent. With it, mismatches show up in the editor and `tsc --noEmit --allowJs --checkJs`.

## Common tags

- `@param {Type} name` describes a parameter.
- `@returns {Type}` describes the return type.
- `@type {Type}` annotates a variable or expression: `/** @type {string[]} */ ([])`.
- `@typedef {{ id: string, name: string }} User` defines a named type usable elsewhere as `@type {User}`.
- `@template T` introduces a type parameter for generic functions.
- `@satisfies {Type}` (TS 4.9+) checks that a value matches a type without widening.

## Project-wide enable

A `jsconfig.json` with `"checkJs": true` flips on type checking for every `.js` in the project. Combine with `"strict": true` for the same strictness floor as a TypeScript project.

## When to graduate to TypeScript

JSDoc + `@ts-check` is the right floor for small JS-only codebases and for libraries that ship without a build step. Once type definitions reach the size of a small module, or when generic helpers proliferate, the verbose JSDoc syntax stops paying off. Move to TypeScript and let the compiler emit the JS.

## References

- TypeScript handbook, JS in TS: https://www.typescriptlang.org/docs/handbook/intro-to-js-ts.html
- JSDoc supported tags in TS: https://www.typescriptlang.org/docs/handbook/jsdoc-supported-types.html
