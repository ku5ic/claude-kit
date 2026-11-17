# Drawing

## Change or replace

- To keep Claude Code's drawing and change one part, pass `next` a copy of the event with changed `props`.
- `next(e)` resolves to the engine's own node, which a tree may place inside a `Box` beside elements of its own.
- Returning a tree without calling `next` replaces the drawing.
- An `AskUserQuestion` tree holds the engine's node exactly once.
- A tree that doesn't validate gets its hook skipped.

## Surfaces

- Build trees from the table `$.ui.resolve(e)` returns. `e.surface` is `terminal`, `desktop`, `vscode`, or `mobile`.
- Branch on `e.surface` before drawing a surface-bound element: `Raster` and `Image` draw only in the terminal, `Svg` only in Desktop and VS Code, and `Client` only in the terminal and Desktop.
- `Button` children other than one string need 2.1.295. Text size, weight, mono and tabular, and Box step names need 2.1.299.
- `Text` `bold` is deprecated for `weight: 'semibold'`.

## Keys

- Every `Input` and `Select` needs a `key`, and so does a `Button` with no label or more than one string child.
- Give each row's Button in a list its own key.

## Redrawing

- `$.ui.invalidate('ui.render')` redraws at most 10 times a second, 30 in the terminal for the visible pane, the expanded band, and the hint line. Faster calls are coalesced.
- A drawing that reads `$.state` redraws when that value is written, with no `invalidate` call.

## Moving the view

- `$.ui.view({ agentId })` (2.1.300) moves the main view only while the call answers the person's own input, such as a Button press or a typed command. From a timer, a load, or a turn it resolves `{ deny: 'not person-initiated' }`.
- It also denies an agent the tasks list shows no row for.
