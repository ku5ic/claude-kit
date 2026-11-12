# Correctness

- `use(promise)` and `use(context)` on 19+: `use()` may be called inside conditionals and loops, unlike every other hook.
- `use(promise)` suspends until the promise resolves: put a `<Suspense>` boundary above it, and don't branch on the promise's status before the call.

## Rules of Hooks

- Call hooks only at the top level of a function component or a custom hook.
- Never call a hook inside a loop, a condition, a nested function, or a `try`/`catch`/`finally` block, or after a conditional `return`.
- Never call a hook in an event handler, in a class component, or in a function passed to `useMemo`, `useReducer`, or `useEffect`.
- A conditional hook: `failure`. Configure `eslint-plugin-react-hooks` to catch it.

## Dependency arrays

- An effect, memo, or callback: list every reactive value it reads (props, state, anything declared in the component body) in its dependency array. React compares them with `Object.is`.
- Removing a dependency: first prove the value isn't reactive, by moving it outside the component or restructuring. Never suppress the lint rule.

## Keys

- A list: key each item with a stable, unique id from the data (`item.id`), the same across renders.
- Index as key: only on a static, append-only list, never on one that reorders, filters, or inserts.

## Effect cleanup

- An effect that sets up a subscription, timer, event listener, or external connection: return a cleanup that tears it down, symmetrical to the setup.
- Strict mode in development mounts, cleans up, and mounts again: treat a doubled side effect as a missing cleanup, never as a strict-mode quirk to turn off.

## Conditional rendering

- A numeric value before `&&` in JSX (`{items.length && <List items={items} />}` renders `0` on an empty list): compare explicitly, as in `items.length > 0 && ...`, or use a ternary.
- A JSX fallback: `value ?? <Fallback />`, not `value || <Fallback />`, which also swallows `0`, `""`, and `false`.
