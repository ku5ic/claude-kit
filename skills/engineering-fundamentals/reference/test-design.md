# Test design

Applies to tests just written, tests being reviewed, or test coverage being evaluated. The boundary and equivalence-partitioning concepts are from Myers, "The Art of Software Testing" (Wiley, 1979 / revised editions).

- **Behavior, not implementation.** Would the test still pass after a refactor that preserves behavior? Tests that read internal state or assert on call counts of internal helpers are testing implementation.
- **Boundary coverage.** For inputs with a range, edge values are tested: zero, one, max, max+1, empty, null where allowed.
- **Equivalence partitioning.** Distinct input classes have at least one test each: valid, invalid, edge, error path.
- **Negative cases.** At least one test per public surface verifies failure mode (invalid input rejected, error raised, expected exception type).
- **Independence.** Tests do not depend on order; each sets up and tears down its own state.
- **Determinism.** No time, random, or network without explicit control. Intermittent CI failure means broken, not flaky.
- **Worth testing.** Pass-through wrappers, trivial getters, and framework defaults are not worth testing. Coverage of these is noise.

## References

- Myers, G.J. (1979 / revised editions), "The Art of Software Testing", Wiley.
