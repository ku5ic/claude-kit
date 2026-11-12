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

### Left to linters

Checked: 2026-11-11 with ruff 0.17.4 defaults (`uvx ruff check --isolated`, no config, no `--select`; the default `PT` set is `PT010`, `PT014`, `PT020`, `PT025`, `PT026`, `PT031`), ESLint 10.14.0 with `js.configs.recommended` from `@eslint/js` 10.0.1, and TypeScript 7.0.2 (`tsc --strict --noEmit`), on probe `test_*.py`, `.test.js`, and `.test.ts` files. No rule was dropped, because no default check reported any test-patterns rule.

Not caught by defaults, so kept: an unawaited `expect(...).resolves`, `findBy*`, or `waitFor` (neither ESLint nor tsc flags a floating promise), a `.then` chain in a test, `findBy*` inside `waitFor`, a mutable variable reset in `beforeEach`, a `unittest.TestCase` subclass in a pytest codebase (`PT009` and `PT027` are off), `parametrize` without `id`s, a `session`-scoped fixture holding mutable state, `os.environ` set without `monkeypatch`, unseeded `random` and `time.time()` in a test (`S311` is off), a real network call in a test, an unawaited coroutine in an async test.

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

- fast-check for JavaScript / TypeScript, shrinking a failing case to a minimal counterexample: https://github.com/dubzzz/fast-check
- hypothesis for Python: https://hypothesis.readthedocs.io/en/latest/

## reference/principles.md

### Claims

- Martin Fowler's Test Pyramid and its granularity quote: https://martinfowler.com/articles/practical-test-pyramid.html
- Kent C. Dodds's Testing Trophy and its confidence quote: https://kentcdodds.com/blog/the-testing-trophy-and-testing-classifications

## reference/pytest.md

### References

- pytest: https://docs.pytest.org/en/stable/

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

- Vitest v5.x, a Jest-compatible API, needs only the Vite transform pipeline: https://vitest.dev/
- Jest as the default for a project without a Vite-based build: https://jestjs.io/
- MSW intercepts with a Service Worker in the browser and a Node interceptor in tests: https://mswjs.io/
