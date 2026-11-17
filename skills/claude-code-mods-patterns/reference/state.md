# State

## Where a value lives

- A module variable is lost on every reload of the mod. A value a drawing or a later hook reads goes in `$.state`.
- `$.state` survives a reload and resets on `/clear`, `/resume`, and `/branch`.
- `$.store` persists across sessions and is shared by all of them. It isn't atomic.

## The contract

- Declare each value under the plugin's name in `interface PluginState`, such as `{ counter: { count: number } }` for plugin `counter`, inside `declare module 'claude-code'`, in a `.d.ts` that `plugin.json` names under `types`.
- The module imports its value types from that file. That import is what brings the declaration into the type check.
- A reference's `plugin` and `key` are literals in a `const`, such as `{ plugin: 'counter', key: 'count' } as const`. Validate fails a reference it can't read statically.
- `StateFamily<T>` holds one value per `id`. A reference to a member spreads the family's const and adds `id`.
- `Shaped<T>`, with `atom(ref, initial, { shape })`, lets new code decline a value that older code stored. Bump the shape tag when the value's fields change.

## Reading and writing

- `read($, atom)` or `$.state.get(ref)` inside a `ui.render` hook subscribes that drawing.
- A render hook never writes. Write from a handler closure, such as `onPress`, or from another event.
- `update($, ref, fn)` reads, applies `fn`, and writes with `ifVersion`, retrying on a miss. Use it wherever two writes can race, and never write a value a closure captured at draw time.
- Only the owning plugin writes its values. Another plugin changes one by hooking `state.set`.
