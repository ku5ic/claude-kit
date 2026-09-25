# Structure

## Controlled vs uncontrolled

Pick one per field. Controlled: the value lives in React state, the input reads from state and writes via `onChange`. Uncontrolled: the value lives in the DOM, React reads it via a ref when needed (or on submit). Mixing them on the same field (defaulting from props but updating in state with no sync) is a bug generator: late prop changes stop being reflected, but the user still expects them to.

For most forms, controlled is the default. Reach for uncontrolled when the value is only inspected on submit (file inputs, third-party widgets) and continuous re-rendering on each keystroke is unwanted.

## Derived state belongs in render

If a value is computed from props or other state, compute it during render. Do not mirror props into `useState` and sync them with an effect:

```jsx
// failure: derived state mirrored into state, synced via effect
const [fullName, setFullName] = useState("");
useEffect(() => {
  setFullName(firstName + " " + lastName);
}, [firstName, lastName]);

// correct: derived during render
const fullName = firstName + " " + lastName;
```

The React docs put it bluntly: if it can be calculated from existing props or state, do not put it in state. For expensive derivations, wrap in `useMemo`.

## Prop drilling threshold

Drilling a prop through one or two layers is fine; readers can follow it. Past that, the cost-benefit shifts:

- Prefer composition (pass children, pass slot props, render-prop) when the dependency is structural.
- Reach for context when the dependency is genuinely cross-cutting (theme, current user, auth tokens).
- Avoid lifting state to a far ancestor just to drill it back down.

The "two levels" figure is a working heuristic; the real test is "does an unrelated reader of this code need to understand five files to know where this prop comes from".

## `useEffect` is for synchronizing with external systems

Effects synchronize React state with something outside React: a subscription, a browser API, a third-party widget, a network connection. They are an escape hatch.

Do not use effects for:

- Computing derived values (use render).
- Handling user-event-driven logic (use event handlers).
- Posting analytics on click (use the click handler).
- Resetting state when a prop changes (use the `key` prop to remount).

The "You Might Not Need an Effect" page on react.dev catalogs the common misuses; if a new effect is being added, check that page first.

## React 19: form actions and `useActionState`

Form submission and other mutations can use Actions: an async function passed as `<form action={fn}>` or `useActionState(fn, initial)`. Actions auto-manage `isPending`, integrate with `useFormStatus` in nested controls, and reset the form on success. Use them when a mutation flows through React state; keep manual `fetch` + `useState` for cases that are not form-shaped.

## References

- "You Might Not Need an Effect": https://react.dev/learn/you-might-not-need-an-effect
- React forms: https://react.dev/reference/react-dom/components/form
- `useActionState`: https://react.dev/reference/react/useActionState
- `useFormStatus`: https://react.dev/reference/react-dom/hooks/useFormStatus
