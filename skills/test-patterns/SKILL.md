---
name: test-patterns
description: Testing patterns for test design, fixtures, mocking, async, and coverage. Use for test configs, test directories, a failing test, or testing questions in any stack.
---

# Test patterns

Apply the runner-specific reference matching the detected test runner. Principles apply to all stacks.

## Reference files

| File                                                                     | Covers                                                                             |
| ------------------------------------------------------------------------ | ---------------------------------------------------------------------------------- |
| [reference/anti-patterns.md](reference/anti-patterns.md)                 | Severity-labeled anti-patterns: implementation details, async pitfalls, mock abuse |
| [reference/principles.md](reference/principles.md)                       | Test design principles + Test Pyramid (Fowler) vs Testing Trophy (Dodds)           |
| [reference/vitest-and-jest.md](reference/vitest-and-jest.md)             | Shared Jest / Vitest patterns, runner choice, MSW for HTTP mocking                 |
| [reference/react-testing-library.md](reference/react-testing-library.md) | Query priority, full priority order, `getBy` vs `queryBy` vs `findBy` semantics    |
| [reference/playwright.md](reference/playwright.md)                       | Playwright patterns, page object, property-based testing (fast-check, hypothesis)  |
| [reference/pytest.md](reference/pytest.md)                               | pytest fixtures, parametrize, pytest-django                                        |
| [reference/coverage-and-skip.md](reference/coverage-and-skip.md)         | Coverage discipline, what to skip, output expectations                             |
