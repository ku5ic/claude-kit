# /audit perf

Performance audit focused on statically detectable issues. Arguments: `<file, directory, area name, or a link to an external tracker/doc>`.

## Procedure

1. Stack is in the `<repo-context>` block from the `SubagentStart` hook. Get the scratch directory via `scratch-dir.sh`.
2. Load the patterns skill for the detected stack (react-patterns, django-patterns, etc.) for the anti-pattern reference.
3. Review the target across these categories:
   - Ground each candidate in this project's own precedent before including it.
   - An established pattern is one that repeats consistently elsewhere in the codebase and is not flagged as a problem by CLAUDE.md or existing tests.
   - An established pattern is a deliberate tradeoff, not a finding.
   - Exception: flag it anyway if it is measurably worse at this location than elsewhere.
4. Skip categories with no findings.

### Frontend (React and Next.js)

- Unnecessary re-renders: inline object or function props to memoized components, context value churn, parent re-renders that include large subtrees
- Missing memoization where it actually matters (measurably expensive computation, or stable reference needed by memo child)
- Key stability on lists that reorder
- Effect waterfalls: multiple sequential effects that could be one or parallel
- Client bundle bloat: client components importing large libraries, server-only logic leaking to client via shared utilities, unused but imported modules
- Next.js: unintended client component propagation, wrong cache or revalidate config for correctness, missing `next/image` or `next/font`
- Core Web Vitals where statically inferable: CLS from missing dimensions, LCP from above-the-fold client-side rendering

### Backend (Django, Node APIs)

- N+1 queries: missing `prefetch_related`, `select_related`, or ORM equivalent
- Unbounded queries: missing pagination, `.all()` on large tables
- Synchronous I/O in async contexts
- Missing indexes on filtered or ordered columns (static inference from query shape)
- Serialization cost: deep nested serializers, recursive expansions
- Cache misuse: caching with key collisions, caching before query filtering

### General

- Unbounded loops
- Accidental O(n^2) via nested iteration over the same collection
- Reading a whole file when streaming is possible
- Blocking operations in request path

## Verification tools

- Command-level perf checks (build scripts, codegen, test runner startup, CLI tools): measure with `hyperfine`, not eyeballed.
- Render-level perf (React re-renders, bundle cost): measure with React DevTools Profiler or browser perf tools at runtime, not from static analysis.

## Output per finding

- Location
- What: the specific pattern
- Why it matters: estimated impact (render count, query count, payload size)
- Verification: how to measure, if not statically obvious (e.g. "profile with React DevTools, expect re-render on every parent state change")
- Fix: concrete code change or pattern switch

## Output file

Load the report-format skill and use its format. Write to the path `scratch-dir.sh perf <target-slug>` prints. Print the path.

## Rules

- Do not claim measured improvement. This command does not run anything.
- Flag what to measure, not what to assume.
- Ignore micro-optimizations that change code without measurable benefit.
- A pattern that repeats across the codebase as an established choice is not N separate findings; note it once against the shared source and list the consuming locations.
