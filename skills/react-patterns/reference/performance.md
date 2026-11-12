# Performance

- Instant feedback on a mutation (likes, votes, list reorders) on 19+: `useOptimistic(currentValue)`. Its setter overrides the value while an async action is in flight and reverts to the real state when it settles, with no manual rollback.

## Memoization

- `useMemo`, `useCallback`, and `React.memo`: add one only when one of these holds, otherwise leave them off.
  - The consumer is wrapped in `React.memo` or sits in a dependency array, and re-running it on every render is wasteful.
  - The computation is expensive: heavy parsing, large transforms, sync work measured in ms.
  - The value is a `useEffect` or `useMemo` dependency where reference identity matters.
- Memoization whose only reason is "just in case": remove it.
- A `React.memo` child, which compares props shallowly with `Object.is`: lift inline objects, arrays, and arrow functions the parent passes it into `useMemo` or `useCallback`.
- A custom `arePropsEqual`: compare every prop, functions included. Never return `true` while a prop changed.

## React Compiler

- The React Compiler is stable, opt-in, and installed per build tool (Babel plugin, Vite, Next.js 15.3.1+, others). It auto-memoizes components that follow the Rules of React.
- Manual memoization stays valid, and it's necessary in a project without the compiler.
- New code in a project with the compiler: rely on it, not on manual `useMemo` and `useCallback`.
- Existing memoization in a project with the compiler: leave it in place, or test carefully before removing it.
- Adopting the compiler: treat every Rules-of-React violation it reports as a real defect.

## Rendering cost

- A list past about 200 visible rows, or a shorter one with heavy rows (deep nesting, charts, images): virtualize it with `react-window` or `@tanstack/react-virtual`. Below that, keep the plain list.
- Before changing list strategy, profile the real interaction. The 200-row figure is a working judgment.
- `useState(compute())` calls `compute()` on every render: for a non-trivial initial value (parsing localStorage, hydrating a derived structure, walking a data tree), pass the lazy form `useState(() => compute())`.
- A context that bundles rarely changing data (theme, current user) with per-keystroke data (form draft, scroll position): split it into two providers.
- A context that must hold both: use a context selector library, or move the high-frequency state out of context into a store with its own subscriptions.
