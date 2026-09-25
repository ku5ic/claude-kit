# Correctness

Baseline: React 19. Most rules apply unchanged to React 18; differences are called out.

## Rules of Hooks

Hooks may only be called at the top level of a function component or another custom hook. Specifically not allowed:

- Inside loops, conditions, nested functions, or `try`/`catch`/`finally` blocks.
- After a conditional `return`.
- In event handlers, in class components, or in functions passed to `useMemo`, `useReducer`, or `useEffect`.

A conditional hook is `failure`. The `eslint-plugin-react-hooks` rule catches it; configure it.

## Dependency arrays

Every reactive value (props, state, anything declared in the component body) referenced inside an effect, memo, or callback must appear in the dependency array. React compares with `Object.is`. Omitting a dependency creates a stale closure: the effect reads the value from the render that created it, not the current one.

Removing a dependency must be earned by proving the value is not reactive (move it outside the component, or restructure). Suppressing the lint rule is not a fix.

## Key stability for lists

Keys must be stable and unique across renders for the same list. Use a stable id from the data (`item.id`), not the array index. Index-as-key is acceptable only when the list is static and append-only; for any list that reorders, filters, or inserts, index-as-key produces input-state mixups (focus jumps, controlled-input values bleed between rows).

## Effect cleanup

Any subscription, timer, event listener, or external connection set up in an effect must be torn down in the cleanup return. The cleanup runs before the effect re-runs (deps changed) and on unmount, and React docs explicitly call out that the cleanup should be symmetrical to the setup.

In strict mode (development), React mounts -> cleans up -> mounts again to surface missing cleanups. Treat any "doubled side effect" warning as a real cleanup gap, not a strict-mode quirk to disable.

## Conditional rendering pitfalls

`{items.length && <List items={items} />}` renders `0` when `items.length === 0` because `0` is a valid React child. Use an explicit check (`items.length > 0 && ...`) or a ternary. Same trap with any numeric falsy value.

JSX nullish guards: `value ?? <Fallback />` is safer than `value || <Fallback />` for similar reasons; `||` swallows `0`, `""`, and `false`.

## React 19: `use()` for promises and context

`use(promise)` and `use(context)` are stable in React 19. Unlike other hooks, `use()` may be called inside conditionals and loops, because it is a special compiler primitive rather than a hook in the rules sense. `use(promise)` suspends the component until the promise resolves; pair it with a `<Suspense>` boundary upstream.

## References

- React Rules of Hooks: https://react.dev/reference/rules/rules-of-hooks
- `useEffect`: https://react.dev/reference/react/useEffect
- React 19 release: https://react.dev/blog/2024/12/05/react-19
- `use()`: https://react.dev/reference/react/use
