# Decision frameworks

Apply when a change forces a judgment call: whether to extract, whether to test, whether to commit, whether to refactor now or defer.

## When to extract a function or component

Extract when one of:

- Used in 3+ places with the same shape
- Internal complexity makes the surrounding code hard to read
- The unit has a name that makes sense outside its current site

Do not extract for:

- Abstract symmetry
- "It might be reused later"
- Reducing line count

## When to add a test

Add a test when:

- The change touches business logic, validation, auth, or data transformation
- A bug is fixed (regression test)
- A boundary condition exists (null, empty, max, error)

Skip a test when:

- The change is purely cosmetic, structural, or refactor with existing coverage
- The code is a thin pass-through to a library
- The framework already guarantees the behavior

## When to commit

Commit when:

- The change leaves the codebase in a working state
- The change has one logical concern
- A reviewer could understand the diff in under two minutes

Do not commit:

- WIP without explicit `wip:` prefix and intent to amend
- Mixed concerns, even if the diff is small
- Generated files alongside source changes (separate commits)

## When to refactor in place vs. defer

Refactor in place when:

- The current task touches the code anyway
- The fix is local and the test surface is unchanged

Defer when:

- The refactor would expand the diff beyond the original task
- The refactor crosses a layer boundary
- The refactor needs new tests of its own
