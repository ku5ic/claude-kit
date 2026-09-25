# Performance

## Memoization has a cost

`useMemo`, `useCallback`, and `React.memo` add bookkeeping (dependency comparison, cache lookup) and lock the value into the closure so future references must run through React. Reach for them when one of the following is true, otherwise leave them off:

- The downstream consumer is wrapped in `React.memo` or sits in a dep array, and re-running it on every render is wasteful.
- The computation itself is genuinely expensive (heavy parsing, large transforms, sync work measured in ms).
- The value is used as a `useEffect` / `useMemo` dependency where reference identity matters.

The React docs are explicit: indiscriminate memoization makes code less readable without a perf payoff. If the only justification is "just in case", remove it.

## `React.memo` requires referentially-stable props

`memo` compares incoming props with `Object.is` (shallow). Inline object literals, array literals, and arrow functions create a fresh reference on every parent render, so the memoized child re-renders anyway. Audit the parent for `<Child config={{ ... }} onClick={() => ...} />`; lift these into `useMemo` / `useCallback` if the memoization is supposed to bite.

If you supply a custom `arePropsEqual`, you must compare every prop, including functions. Returning `true` while a prop changed produces stale closures inside the child's handlers.

## Large lists: virtualize at scale

Past roughly 200 visible rows, a flat list starts to dominate paint and layout time. Virtualize (`react-window`, `@tanstack/react-virtual`) once you cross that threshold, or earlier if rows are heavy (deep nesting, charts, images). Below it, the simple list is usually correct and easier to maintain.

The 200-row figure is a working judgment, not a measured guarantee; profile the real interaction before changing strategy.

## `useState` lazy initializer

`useState(compute())` calls `compute()` on every render and discards the result after the first; `useState(() => compute())` only calls the function on mount. Use the lazy form when the initial value is non-trivial (parsing localStorage, hydrating a derived structure, walking a data tree).

## Context: split by update cadence

A context value change re-renders every consumer in the subtree. If a single context bundles "rarely changes" data (theme, current user) with "changes per keystroke" data (form draft, scroll position), every consumer pays for both. Split into two providers.

When a single context truly must hold both, consider a context selector library or moving the high-frequency state out of context entirely (e.g. into a store with its own subscription model).

## React Compiler

The React Compiler reaches stable status; it remains opt-in (not enabled by default) and is installed per build tool (Babel plugin, Vite, Next.js 15.3.1+, etc.). Enabling it auto-memoizes components that follow the Rules of React, often replacing manual `useMemo` / `useCallback` for re-render avoidance. Manual memoization remains valid (and necessary for projects that have not adopted the compiler), but in compiler-enabled projects the manual passes can usually be removed.

When you adopt the compiler, treat every Rules-of-React violation it reports as a real defect: the compiler skips files it cannot prove safe.

## React 19: `useOptimistic`

`useOptimistic(currentValue)` returns an immediate optimistic value plus a setter that overrides the value while an async action is in flight, then auto-reverts to the real state on settle. Use it for instant feedback on mutations (likes, votes, list reorders) without manual rollback bookkeeping.

## References

- `React.memo`: https://react.dev/reference/react/memo
- `useState` lazy initializer: https://react.dev/reference/react/useState
- React Compiler introduction: https://react.dev/learn/react-compiler/introduction
- `useOptimistic`: https://react.dev/reference/react/useOptimistic
