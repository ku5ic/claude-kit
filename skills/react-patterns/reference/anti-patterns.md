# Anti-patterns

Severity rubric:

- `failure`: a concrete defect or violation that should not ship.
- `warning`: a smell or pattern that compounds with other findings.
- `info`: a hardening opportunity or note, not a defect.

## `useState` set from props inside an effect

`failure`. Mirroring a prop into local state and syncing it via effect creates a lag (one render of stale value) and conflicts with subsequent prop changes. If the value comes from props, derive it during render or use the `key` prop to remount on identity change.

## `useEffect` running synchronous computation from state

`failure`. Effects run after commit, so the computed value is one render late, and the extra state update triggers an avoidable re-render. Compute during render; wrap in `useMemo` only if the computation is genuinely expensive.

## Inline object or array literal passed to a memoized component

`warning`. `<MemoizedChild config={{ ... }} items={[...]} />` defeats `React.memo` because the literal is a new reference every render. Lift to `useMemo` or hoist to module scope, or remove the `memo` if it never paid off.

## `key={index}` on a list that reorders

`failure`. Index keys map "the third row" to whatever data is currently third, so insertions, deletions, and reorders confuse component identity: focus jumps, controlled-input values bleed, animations replay. Use a stable id from the data.

## `dangerouslySetInnerHTML` with unsanitized input

`failure`. Direct XSS vector. Either sanitize with a library that handles the full HTML parsing surface (DOMPurify) or render as text. Never inject user-derived strings as HTML without sanitization.

## `fetch` in a client component without error handling

`warning`. A network call without a `.catch()` and without a status check renders the happy path until the failure produces an unhandled rejection or a confusing error boundary trip. Either explicitly handle non-2xx and rejection, or use a data layer that owns those cases (TanStack Query, SWR, framework-provided loader).

## Mutating props or state directly

`failure`. `props.user.role = "admin"` and `state.items.push(item)` violate React's assumption of immutability: memoization compares by reference, so mutated objects never trigger re-renders, and shared state across components diverges silently. Always replace, never mutate (`setItems(prev => [...prev, item])`).

## References

- "You Might Not Need an Effect": https://react.dev/learn/you-might-not-need-an-effect
- `React.memo` pitfalls: https://react.dev/reference/react/memo
- React keys: https://react.dev/learn/rendering-lists#keeping-list-items-in-order-with-key
- `dangerouslySetInnerHTML`: https://react.dev/reference/react-dom/components/common#common-props
