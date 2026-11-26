# Testing

## claude plugin test

- `claude plugin test`, given the plugin's root folder, runs the plugin's `*.test.ts` files against the engine. Import `test`, `expect`, `mock`, and `tier` from `claude-code/testing`.
- The test's `$` fires events. Its `on` registers hooks that sit beneath the plugin and stand for the engine.
- A stub answers a `$` call with `{ value }`, or refuses it with `{ deny }`.
- Register those stubs before the first call on `$`. `session.start` doesn't run on its own.
- `mock.clock(on)` controls time, with `advance`, `set`, `settle`, and `sleep`.
- `$.ui.mount({ plugin, component, requestId, props, surface })` mounts a drawing, then `find`, `press`, `input`, and `select` drive it. Mount one per surface the mod branches on.
- The test kit answers `$.state` and `$.ui.invalidate` itself.
- `test(name, { options }, body)` sets the plugin's `userConfig` values. Left out, the manifest's defaults apply.

## claude plugin validate

- It reads the manifest and modules as the engine will, and lists hooks, calls, state reads and writes, and each `gating hook without .catch`. `--strict` fails on its warnings.
- It refuses non-literal event names, `$` stored in a variable or destructured, `$` passed to anything but a top-level function in the same file or, from 2.1.299, an imported function declared with `function`, a dynamic `import()`, a shadowed `on`, and imports other than relative paths and `claude-code`.
- From 2.1.323 it also type-checks TypeScript modules.
