# Anti-patterns

Severity rubric: `skills/report-format`.

## A guard with no `.catch`

`failure`. A guard that throws, times out, or returns the wrong shape is skipped, so what it judges goes ahead. Add `.catch(($, e, next) => next.called ? next(e) : { deny: 'why' })`, with the event's own refusal.

## `$.state` written while drawing

`failure`. A render hook's write is denied. Write from a handler such as `onPress`, or from another event, with `update`.

## Drawing state in a module variable

`warning`. A reload resets it while the drawing still shows the old value. Keep it in `$.state`, declared in the plugin's contract.

## A state reference built at run time

`warning`. Validate can't read a computed `plugin` or `key`, and fails it. Keep them literal in a `const`, and compute only a family member's `id`.

## A surface-bound element drawn on every surface

`warning`. `Raster` and `Image` draw only in the terminal, and `Svg` only in Desktop and VS Code, so an unbranched tree is empty elsewhere. Branch on `e.surface`.

## `$.ui.view` from a timer, a load, or a turn

`warning`. It resolves `{ deny: 'not person-initiated' }`. Call it from a handler that answers the person's own input, such as a Button press.

## A list of Buttons without their own keys

`warning`. Give each row's Button a key of its own, such as the item's id.

## A tree that replaces a site to change one detail

`info`. It drops Claude Code's own drawing of everything else. Pass `next({ ...e, props })` with the one change.

## `setInterval` or `setTimeout` in a module

`failure`. The module has neither. Use `$.clock.every` or `$.clock.after`, started in `session.start`.

## A system-prompt section or tool description changed mid-thread

`info`. The change waits for a new thread or a compaction. Use `breakPromptCache: true` only for what can't wait, since it costs the cache.

## `Text` `bold`

`info`. Deprecated. Use `weight: 'semibold'`.
