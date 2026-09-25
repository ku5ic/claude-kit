# When to split a component

Component splitting is a judgment call. Reach for it when one of these holds:

- **Render function exceeds about 150 lines.** At that size, the component is hard to scan in one screen, and reviewers cannot hold the structure in their head. The 150-line figure is a working heuristic; the real test is whether a new reader can understand the component's purpose at a glance.
- **Two independent concerns re-render at different cadences.** A header that updates on auth changes and a feed that updates on every keystroke do not belong in the same component; the slow part is forced to re-render with the fast part. Splitting and using `React.memo` (or relying on the React Compiler) on the slow side breaks the cascade.
- **A subsection has its own state the parent does not need.** Lift state down: keep the local state inside the smaller component. The parent stops re-rendering on every keystroke or hover, and the new component's API gets simpler.
- **The component is reused in two or more places.** A real second use site is the strongest signal. Two near-copies that look the same but diverge over time are usually worse than one component with two configurations, but only if the configurations are stable.

## When not to split

- "It is getting big" alone is not a reason. Premature splits scatter logic across files and force the reader to chase imports for context that used to live next to its callers.
- "Symmetry" with another component is not a reason. Two components that happen to look similar today often diverge tomorrow.
- "Just in case we reuse it" is not a reason. Wait for the second use site.

The duplication-vs-abstraction tradeoff: three similar-looking blocks in one file is usually better than the wrong abstraction extracted across three files.

## References

- React: passing data as children: https://react.dev/learn/passing-props-to-a-component
- React: extracting JSX: https://react.dev/learn/your-first-component
