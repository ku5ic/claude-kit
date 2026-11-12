# When to split a component

- A render function past the function-size threshold in `skills/engineering-fundamentals/reference/metric-thresholds.md` whose purpose a new reader can't see at a glance: split it.
- Two independent concerns that re-render at different cadences, such as a header that updates on auth changes and a feed that updates on every keystroke: split them, and put `React.memo` (or the React Compiler) on the slow side.
- A subsection with state the parent doesn't need: move it into its own component and keep that state there.
- Split out a component used in two or more places. A real second use site is the strongest signal.
- Two near-copies of one component: merge them into one component with two configurations, by DRY in `skills/kit-rules/reference/change.md` section 8.

## When not to split

- Never split only because the component is getting big.
- Never split for symmetry with another component.
- Never split for a reuse or an abstraction that doesn't exist yet: YAGNI and DRY in `skills/kit-rules/reference/change.md` section 8.
