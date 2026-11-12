# Red-green-refactor

The cycle for adding behavior a test can observe: one behavior per turn, and each turn ends with every test green.

- The cycle applies to new behavior and bug fixes with a test seam. A pure refactor has no red step; the existing tests are its guard (`skills/kit-rules/reference/change.md` section 6).

<!-- chat: skip -->

A change no test can reach gets the runtime pass instead (`skills/kit-rules/reference/verify.md` section 2).

<!-- chat: end -->

- Red. Write the smallest test for the next behavior and run it. It fails on its assertion. A failure from setup, an import, or a compile error proves nothing yet; fix that and rerun. A test that passes before the change is testing something else. For a bug fix, the red test is the reproduction.
- Green. Write the least code that makes it pass, sized per `skills/kit-rules/reference/change.md` section 1, and run it with its neighbors. No cleanup yet.
- Refactor. Required: first green is never done. Re-read what green added against `skills/kit-rules/reference/change.md` sections 4, 8, 9, and 10 (dead code, duplication, over-engineering, less code), then improve its shape with every test green and the tests unchanged, per section 6, running them after each move. End by naming what the pass changed, or why nothing needed to.
- Then the next behavior.

<!-- chat: skip -->

How to design the test itself is `test-patterns`.

<!-- chat: end -->

- In review, a bug fix whose test passes against the code before the fix: `warning`.
