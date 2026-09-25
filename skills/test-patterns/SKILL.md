---
name: test-patterns
description: Testing conventions - test design, fixtures, mocking, query priority, async handling, behavior-vs-implementation tests, boundary coverage, and what to skip. Use whenever the project contains a test config file (`vitest.config.*`, `jest.config.*`, `playwright.config.*`, `pytest.ini`, `conftest.py`) or a test directory (`tests/`, `__tests__/`, `spec/`), OR the user asks about tests at any level, mocking, fixtures, coverage, or a failing test, in any stack and even if a framework is not named.
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

## References

- Vitest: https://vitest.dev/
- Jest: https://jestjs.io/
- React Testing Library: https://testing-library.com/docs/react-testing-library/intro/
- Testing Library guiding principles: https://testing-library.com/docs/guiding-principles/
- Playwright: https://playwright.dev/
- pytest: https://docs.pytest.org/en/stable/
- pytest-django: https://pytest-django.readthedocs.io/
- Kent C. Dodds (Testing Trophy): https://kentcdodds.com/blog/the-testing-trophy-and-testing-classifications
- Martin Fowler (Test Pyramid): https://martinfowler.com/articles/practical-test-pyramid.html
- MSW: https://mswjs.io/
- fast-check: https://github.com/dubzzz/fast-check
- hypothesis: https://hypothesis.readthedocs.io/en/latest/

## Version notes

Checked: 2026-09-12 against the migration and release pages listed above

- Vitest 4.x (Context7 /vitest-dev/vitest); the migration guide at vitest.dev lists for the next major: `vi.mock` / `vi.unmock` / `vi.hoisted` must be module-scoped, `clearMocks` defaults to true, automocked modules return `undefined` in browser mode. None of the patterns in `reference/vitest-and-jest.md` depend on the old behavior.
- Jest 30 (2025-06): `--testPathPattern` renamed `--testPathPatterns`; `expect` aliases removed. pytest 9 (2026-06): fixture visibility now decides precedence over registration order; `PytestRemovedIn9Warning` is an error.
- MSW 2: `http` / `HttpResponse` replace `rest` and the `req, res, ctx` signature. Unchanged.
