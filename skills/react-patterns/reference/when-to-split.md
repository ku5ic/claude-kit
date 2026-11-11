# When to split a component

- A render function past about 150 lines: split it. The 150-line figure is a working heuristic; the real test is whether a new reader sees the component's purpose at a glance.
- Two independent concerns that re-render at different cadences, such as a header that updates on auth changes and a feed that updates on every keystroke: split them, and put `React.memo` (or the React Compiler) on the slow side.
- A subsection with state the parent doesn't need: move it into its own component and keep that state there.
- Split out a component used in two or more places. A real second use site is the strongest signal.
- Two near-copies of one component: merge them into one component with two configurations, but only if those configurations are stable.

## When not to split

- Never split only because the component is getting big.
- Never split for symmetry with another component.
- Never split "just in case we reuse it". Wait for the second use site.
- Three similar-looking blocks in one file usually beat the wrong abstraction extracted across three files.
