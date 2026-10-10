# Red-green-refactor

The cycle for adding behavior a test can observe: one behavior per turn, and each turn ends with every test green.

It applies to new behavior and bug fixes with a test seam. A pure refactor has no red step, since the existing tests are its guard (`rules/change.md` section 6). A change no test can reach gets the runtime pass instead (`rules/verify.md` section 2).

- **Red.** Write the smallest test for the next behavior and run it. It fails on its assertion, because the behavior is missing. A failure from setup, an import, or a compile error proves nothing yet: fix that and rerun. A test that passes before the change is testing something else. For a bug fix, the red test is the reproduction.
- **Green.** Write the least code that makes it pass, sized per `rules/change.md` section 1, and run it with its neighbors. No cleanup yet.
- **Refactor.** Improve the shape with every test green and the tests unchanged, per `rules/change.md` section 6, running them after each move.

Then the next behavior. How to design the test itself is `test-patterns`.

In review, a bug fix whose test passes against the code before the fix is a `warning`: nothing shows it reproduces the bug.
