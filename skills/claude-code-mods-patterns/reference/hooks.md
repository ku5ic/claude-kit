# Hooks

## The hook function

- The module exports `register(on, options)`, and `hooks/hooks.json` names it under `modules`. `options` holds the plugin's `userConfig` values.
- Every hook is `($, e, next)`. `e` is deeply frozen: observe with `return next(e)`, rewrite with `next({ ...e, field })`, and answer for the event by returning without calling `next`.
- Registering one event twice without a matcher fails the load. A matcher is a value, an array, or a regex per field.
- Hooking another mod's `ui.press`, `ui.input`, `ui.select`, `ui.focus`, or `ui.render` needs a `plugin` matcher.

## When a hook fails

- A hook that throws, times out, or returns the wrong shape is skipped, and the chain goes on without it.
- A guard, a hook whose answer refuses something, carries `.catch(($, e, next) => next.called ? next(e) : { deny: 'why' })`. Without it, what the guard judges goes ahead when the guard fails.
- In a `.catch`, `next(e)` replays what the hook's own `next` call settled to, so nothing beneath runs twice.
- Each event has its own refusal: `{ deny }` at `tool.call` and `config.set`, `{ drop }` at `prompt.submit`, `{ decision: 'deny' }` at `tool.check`, `{ refuse }` at `plugin.register`.

## Budgets

- A hook has 10 s and a `.catch` 1 s. Time inside `next` or a `$` call doesn't count, but awaiting the hook's own promise or a `$.clock.sleep` does.
- `next.signal` aborts when the dispatch is abandoned. Anything the hook started stops on it.

## Work that outlives a hook

- Register commands and tools and start timers in a `session.start` hook. It runs again on a reload, not after `/clear`.
- The module has no `setInterval` or `setTimeout`. Use `$.clock.every` and `$.clock.after`; a reload stops them.
- `$.tool.register` in `session.start` lists the tool under the plugin's name, so plugin `kit`'s tool `notes` is `mcp__kit__notes`.
- Serve a registered tool from a `tool.run` or `tool.call` hook matched on that name.
- `$.command.register` throws on a built-in command's name, which skips the rest of that `session.start` hook.

## The prompt cache

- A changed system-prompt section or tool description waits for a new thread or a compaction. `breakPromptCache: true` (2.1.321) sends it now, at the cost of the cache.
- New context for the model goes in a new row with `$.session.append` and `type: 'user'`, never in a rewrite of what a thread already sent.
