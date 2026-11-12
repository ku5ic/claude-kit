---
name: test-patterns
description: Testing patterns for test design, fixtures, mocking, async, and coverage. Load before any edit to a test, test helper, fixture, or test config, however small, in any stack, and for testing questions.
---

# Test patterns

Apply the runner-specific reference matching the detected test runner. Principles apply to all stacks.

## Reference files

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                                             | Read                                                                     |
| ------------------------------------------------------------------ | ------------------------------------------------------------------------ |
| Writing, changing, or reviewing tests                              | [reference/anti-patterns.md](reference/anti-patterns.md)                 |
| Designing a test, or choosing what level to test at                | [reference/principles.md](reference/principles.md)                       |
| Writing a Jest or Vitest test, picking a runner, or mocking HTTP   | [reference/vitest-and-jest.md](reference/vitest-and-jest.md)             |
| Querying the DOM in a React Testing Library test                   | [reference/react-testing-library.md](reference/react-testing-library.md) |
| Writing a Playwright test, a page object, or a property-based test | [reference/playwright.md](reference/playwright.md)                       |
| Writing a pytest test, a fixture, or a parametrized case           | [reference/pytest.md](reference/pytest.md)                               |
| Deciding what to cover or skip, or reporting test output           | [reference/coverage-and-skip.md](reference/coverage-and-skip.md)         |
