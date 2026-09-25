# Verification and validation

Applies during review or post-implementation reflection. The V&V distinction is canonical from Boehm, "Software Engineering Economics" (Prentice-Hall, 1981).

- **Verification (building it right).** Does the code implement what the plan or spec said? If the plan moved while implementing, was the plan updated? Drift between plan and code is `warning`.
- **Validation (building the right thing).** Does the change actually solve the underlying problem? A change that is technically correct but misses the user's actual need fails validation, regardless of how clean the code is.
- **Traceability.** Can each behavior change be traced to a line in the spec, plan, or ticket? Behavior in code without a corresponding requirement is a `warning`; surface it.

## Failure modes worth recognizing

- A unit test that perfectly matches a buggy specification will pass verification but fail validation. The test confirms the code does what the spec said; the spec does not solve the user's problem.
- A change that solves the problem in the ticket but breaks an unstated invariant of an adjacent feature passes validation in isolation but fails when the system is taken as a whole.
- A change that the team agrees solves the problem but lacks any test or check passes neither: there is no way to verify the implementation matches the intent.

The practical instruction: at review time, ask both "does this code match what we said we'd build?" (verification) and "does what we said we'd build solve the actual problem?" (validation). Either question failing is a finding worth surfacing.

## References

- Boehm, B.W. (1981), "Software Engineering Economics", Prentice-Hall (Englewood Cliffs, NJ; ISBN 0-13-822122-7). Introduced the COCOMO cost model and the canonical V&V framing.
