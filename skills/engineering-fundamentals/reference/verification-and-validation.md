# Verification and validation

Two review questions; either failing is a finding. When each check runs is `rules/verify.md`.

- **Verification: built it right.** Does the code do what the plan or spec said? If the plan moved during implementation, was it updated? Drift between plan and code is a `warning`.
- **Validation: built the right thing.** Does the change solve the underlying problem? A technically correct change that misses the user's actual need fails, however clean the code.
- **Traceability.** Each behavior change traces to a line in the spec, plan, or ticket. Behavior with no matching requirement is a `warning`; surface it.

Failure modes worth recognizing:

- A test that perfectly matches a buggy spec passes verification and fails validation.
- A change that solves the ticket but breaks an unstated invariant of an adjacent feature passes validation in isolation and fails as a whole.
- A change everyone agrees solves the problem, with no test or check, passes neither: nothing shows the implementation matches the intent.
