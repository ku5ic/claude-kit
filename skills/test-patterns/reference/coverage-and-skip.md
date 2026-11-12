# Coverage, what to skip, and output

## Coverage

- Treat coverage as a signal, not a goal. 100% on generated code is meaningless; focus on core logic and branching.
- Flag uncovered branches in code with business rules, auth, or payment flows.
- Scale test depth with criticality, not with how much can be tested.
- Business rules, auth, payment, and security boundaries get full boundary, negative, and equivalence-class coverage.
- A thin wrapper, a single-caller internal helper, or plain display logic gets one happy-path test, not the full checklist.

## What to skip as not worth testing

- Direct pass-through wrappers around library calls with no logic.
- Trivial getters and setters.
- Framework defaults, such as React re-rendering on state change.

## Output when adding tests

- A new test file mirrors the source path.
- Each test describes a behavior, not a function.
- A public function with validation gets at least one negative case.
- Run the new tests before finishing, and report pass, fail, and the coverage delta if measurable.
