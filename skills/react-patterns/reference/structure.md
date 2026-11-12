# Structure

- A mutation that flows through React state, form submission included, on 19+: an Action, an async function passed as `<form action={fn}>` or to `useActionState(fn, initial)`. Actions manage `isPending`, work with `useFormStatus` in nested controls, and reset the form on success.
- A mutation that isn't form-shaped: keep manual `fetch` plus `useState`.

## Controlled vs uncontrolled

- Each field: pick controlled (the value lives in React state, written via `onChange`) or uncontrolled (the value lives in the DOM, read through a ref when needed or on submit).
- Never mix the two on one field, such as defaulting from props and then updating in state with no sync.
- Most forms: controlled. Use uncontrolled when the value is only read on submit (file inputs, third-party widgets) and a re-render on each keystroke is unwanted.

## Derived state

- Compute a value derived from props or other state during render. Never mirror it into `useState` and sync it with an effect.
- An expensive derivation: wrap it in `useMemo`.

## Prop drilling

- Drilling a prop through one or two layers is fine.
- Deeper, with a structural dependency: use composition (children, slot props, render props).
- Deeper, with a cross-cutting dependency (theme, current user, auth tokens): use context.
- Never lift state to a far ancestor just to drill it back down.
- Two levels is a working heuristic. The real test is whether an unrelated reader needs five files to learn where a prop comes from.

## Effects

- `useEffect`: only to synchronize with a system outside React, such as a subscription, a browser API, a third-party widget, or a network connection.
- A derived value: compute it in render, not in an effect.
- Put logic driven by a user event, analytics on a click included, in the event handler.
- To reset state when a prop changes, remount with the `key` prop.
- Before adding an effect, check react.dev's "You Might Not Need an Effect".
