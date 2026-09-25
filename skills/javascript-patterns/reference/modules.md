# Modules: ESM and CJS

## ESM is the modern default

For new code, prefer ECMAScript modules (`import` / `export`). CommonJS (`require` / `module.exports`) remains supported for legacy code and packages, but Node now treats ESM as the modern path: dynamic `import()`, top-level `await`, and standardized syntax all live on the ESM side.

## `package.json` `"type"` field

`"type"` controls how Node interprets `.js` files in the package folder and its subfolders, until another `package.json` overrides it.

- `"type": "module"` -> `.js` is parsed as ESM.
- `"type": "commonjs"` (or omitted) -> `.js` is parsed as CJS.

The setting applies recursively until another `package.json` is found.

## File extensions force module type

- `.mjs` is always ESM, regardless of the closest `"type"`.
- `.cjs` is always CJS, regardless of the closest `"type"`.

Use the explicit extension when a single file inside a package needs the opposite mode.

## Top-level `await` is ESM-only

`await` at the top level of a module body works only in ESM. Inside CJS files, top-level `await` is a syntax error. If a top-level `await` never resolves, the Node process exits with status code 13.

## Dynamic `import()` works from both sides

`import()` returns a Promise and works in both CJS and ESM. CJS code can use it to load ESM modules asynchronously:

```js
const mod = await import("./esm-module.mjs");
```

`require()` can load synchronous ESM (no top-level `await`), but dynamic `import()` is the safer cross-mode bridge.

## Dual-package hazard

Shipping the same package as both CJS and ESM via the `"exports"` `import` / `require` conditions can produce two parallel instances of the module's state. Consumers that mix `require` and `import` get two copies, which breaks singletons and instance checks. Use one entry point per package, or document the constraint.

## Mixing `require` and `import` in the same file

`failure`. The two systems are not interchangeable inside a single module. Pick one per file. To bridge from CJS to ESM, use dynamic `import()`. To bridge from ESM to CJS, use `module.createRequire()`.

## References

- Node.js packages: https://nodejs.org/api/packages.html
- Node.js ESM: https://nodejs.org/api/esm.html
