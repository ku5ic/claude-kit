# react-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- React docs: https://react.dev/
- Rules of React: https://react.dev/reference/rules
- React 19 release notes: https://react.dev/blog/2024/12/05/react-19
- React Compiler: https://react.dev/learn/react-compiler/introduction
- "You Might Not Need an Effect": https://react.dev/learn/you-might-not-need-an-effect
- Dan Abramov on React internals: https://overreacted.io/
- Mark Erikson on React state management: https://blog.isquaredsoftware.com/

### Version notes

Checked: 2026-11-10 against https://react.dev/versions, https://react.dev/blog/2026/09/09/react-19-3 and https://react.dev/blog/2025/10/07/react-compiler-1

- 19.3 (2026-09-09): a minor release, not a patch. Adds View Transitions, Fragment Refs, `browser()`, and Trusted Types; not yet guidance here. Corrected the compiler advice on removing existing memoization.
- 19.0 (2024-12-05): Actions, `use`, `useActionState`, `useFormStatus`, `useOptimistic`. Verify against the per-hook page on react.dev, not older blog posts.
- React Compiler: stable, opt-in, installed per build tool; not present in every project.

### Left to linters

Checked: 2026-11-11 with ESLint 10.14.0 and `js.configs.recommended` from `@eslint/js` 10.0.1 (JSX parsing on, no plugins), and TypeScript 7.0.2 (`tsc --strict --noEmit`, plus `--jsx react-jsx` so the probe parses) with `@types/react`, on a probe directory. No rule was dropped, because no default check reported any React rule.

Not caught by defaults, so kept: a conditional hook, a hook after an early return, a missing effect dependency, `useState` synced from props in an effect, mutated props and state, `useState(compute())`, an effect with no cleanup, `fetch` with no `.catch()`, `items.length &&` in JSX, `||` as a JSX fallback, `key={index}`, `dangerouslySetInnerHTML`, inline literals passed to a `memo` child.

## reference/anti-patterns.md

### References

- "You Might Not Need an Effect": https://react.dev/learn/you-might-not-need-an-effect
- `React.memo` pitfalls: https://react.dev/reference/react/memo
- React keys: https://react.dev/learn/rendering-lists#keeping-list-items-in-order-with-key
- `dangerouslySetInnerHTML`: https://react.dev/reference/react-dom/components/common#common-props

## reference/correctness.md

### References

- React Rules of Hooks: https://react.dev/reference/rules/rules-of-hooks
- `useEffect`: https://react.dev/reference/react/useEffect
- React 19 release: https://react.dev/blog/2024/12/05/react-19
- `use()`: https://react.dev/reference/react/use

### Claims

- `use(promise)` and `use(context)` on 19+, callable inside conditionals and loops: https://react.dev/reference/react/use

## reference/performance.md

### References

- `React.memo`: https://react.dev/reference/react/memo
- `useState` lazy initializer: https://react.dev/reference/react/useState
- React Compiler introduction: https://react.dev/learn/react-compiler/introduction
- `useOptimistic`: https://react.dev/reference/react/useOptimistic

## reference/structure.md

### References

- "You Might Not Need an Effect": https://react.dev/learn/you-might-not-need-an-effect
- React forms: https://react.dev/reference/react-dom/components/form
- `useActionState`: https://react.dev/reference/react/useActionState
- `useFormStatus`: https://react.dev/reference/react-dom/hooks/useFormStatus

## reference/when-to-split.md

### References

- React: passing data as children: https://react.dev/learn/passing-props-to-a-component
- React: extracting JSX: https://react.dev/learn/your-first-component
