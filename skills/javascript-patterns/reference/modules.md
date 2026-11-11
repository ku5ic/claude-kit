# Modules: ESM and CJS

- New code: ESM (`import`/`export`). CJS (`require`/`module.exports`) stays for legacy code and packages.
- `package.json` `"type": "module"`: `.js` parses as ESM. `"commonjs"` or no field: CJS. It applies to subfolders until another `package.json`.
- One file that needs the other mode: `.mjs` is always ESM, `.cjs` always CJS.
- Top-level `await`: ESM only, a syntax error in CJS. If it never resolves, Node exits with code 13.
- CJS loading ESM: `await import("./esm-module.mjs")`. `require()` loads synchronous ESM (no top-level `await`), but `import()` is the safer bridge.
- ESM loading CJS: `module.createRequire()`.
- One file: `require` or `import`, never both. Mixing them is `failure`.
- Shipping CJS and ESM through the `"exports"` `import`/`require` conditions: consumers that mix both get two copies of the module's state, which breaks singletons and instance checks. Ship one entry point, or document the constraint.
