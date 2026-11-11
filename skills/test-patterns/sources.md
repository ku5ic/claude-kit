# test-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

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

### Version notes

Checked: 2026-11-10 against https://pypi.org/project/pytest/, https://docs.pytest.org/en/stable/changelog.html, https://vitest.dev/guide/migration, https://mswjs.io/docs/migrations/2.x-to-3.x and the npm registry

- Vitest 5 (5.0.0 on 2026-09-03, latest 5.1.0): `vi.mock` / `vi.unmock` / `vi.hoisted` must be top-level, `clearMocks` defaults to true, automocked modules return `undefined` in browser mode. None of the patterns in `reference/vitest-and-jest.md` depend on the old behavior.
- MSW 3 (latest 3.0.4): `http` and `HttpResponse` now import from `msw/http`. MSW 2 had replaced `rest` and the `req, res, ctx` signature with them.
- Jest 30 (2025-06): `--testPathPattern` renamed `--testPathPatterns`; `expect` aliases removed. pytest 9 (2025-11; 9.2.0 current): `PytestRemovedIn9Warning` is an error; since 9.1.0, fixture visibility decides precedence over registration order.

## reference/anti-patterns.md

### References

- Testing Library guiding principles: https://testing-library.com/docs/guiding-principles/
- Kent C. Dodds (avoid testing implementation details): https://kentcdodds.com/blog/testing-implementation-details
- Vitest async handling: https://vitest.dev/guide/common-errors
- MSW: https://mswjs.io/

## reference/playwright.md

### References

- Playwright: https://playwright.dev/
- Playwright locators: https://playwright.dev/docs/locators
- fast-check: https://github.com/dubzzz/fast-check
- hypothesis: https://hypothesis.readthedocs.io/en/latest/

### Claims

- Property-based testing for invariants, **fast-check** for JavaScript / TypeScript. QuickCheck-in...: https://github.com/dubzzz/fast-check
- Property-based testing for invariants, **hypothesis** for Python. The canonical property-based l...: https://hypothesis.readthedocs.io/en/latest/

## reference/principles.md

### Claims

- Test pyramid vs testing trophy, **Martin Fowler's Test Pyramid**:: https://martinfowler.com/articles/practical-test-pyramid.html
- Test pyramid vs testing trophy, **Kent C. Dodds's Testing Trophy**:: https://kentcdodds.com/blog/the-testing-trophy-and-testing-classifications

## reference/pytest.md

### References

- pytest: https://docs.pytest.org/en/stable/
- pytest-django: https://pytest-django.readthedocs.io/
- Factory Boy: https://factoryboy.readthedocs.io/en/stable/

## reference/react-testing-library.md

### References

- Testing Library queries: https://testing-library.com/docs/queries/about/
- Testing Library guiding principles: https://testing-library.com/docs/guiding-principles/
- userEvent: https://testing-library.com/docs/user-event/intro

## reference/vitest-and-jest.md

### References

- Vitest: https://vitest.dev/
- Jest: https://jestjs.io/
- Jest matchers: https://jestjs.io/docs/expect
- MSW: https://mswjs.io/

### Claims

- Choosing between Vitest and Jest, **Vitest**: Vite-native test runner with a Jest-compatibl...: https://vitest.dev/
- Choosing between Vitest and Jest, **Jest**: the older, broader-ecosystem incumbent. Still a...: https://jestjs.io/
- MSW for HTTP mocking, **Mock Service Worker (MSW)**: intercepts at the Service...: https://mswjs.io/
