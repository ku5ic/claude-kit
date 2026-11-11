# Anti-patterns

Severity levels: the report-format skill.

- `useState` set from a prop inside an effect: `failure`. Derive the value during render, or remount on identity change with the `key` prop.
- An effect that computes a value synchronously from state: `failure`. Compute it during render; add `useMemo` only if the computation is expensive.
- An inline object or array literal passed to a memoized component, as in `<MemoizedChild config={{ ... }} items={[...]} />`: `warning`. Lift it into `useMemo`, hoist it to module scope, or drop the `memo` if it never paid off.
- `key={index}` on a list that reorders: `failure`. Use a stable id from the data.
- `dangerouslySetInnerHTML` with unsanitized input: `failure`. Render it as text, or sanitize it with DOMPurify first. The rest of the XSS surface (URL schemes, open redirects) is in security-patterns' `reference/frontend.md`.
- `fetch` in a client component with no `.catch()` and no status check: `warning`. Handle non-2xx responses and rejection explicitly, or use a data layer that owns them (TanStack Query, SWR, a framework loader).
- Mutating props or state directly, as in `props.user.role = "admin"` or `state.items.push(item)`: `failure`. Replace, never mutate: `setItems(prev => [...prev, item])`.
