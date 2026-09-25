# Principles (all stacks)

## Core principles

- Test behavior, not implementation. If refactoring internals breaks the test, the test is too coupled.
- One clear assertion per test where possible. Multiple related assertions are fine; sprawling assertions are not.
- Arrange, act, assert. Visible in the test structure.
- Fixtures over factories over repetition. Extract when used three or more times.
- Name tests as full sentences describing the expected behavior, not the method under test.
- Flaky tests are failures. Fix or delete, never retry loop.

## Test pyramid vs testing trophy

Two valid models for thinking about test distribution. They are not contradictory; they reflect different software architectures.

**Martin Fowler's Test Pyramid** (https://martinfowler.com/articles/practical-test-pyramid.html):

- Many fast unit tests at the base.
- Some integration tests in the middle.
- Few end-to-end tests at the apex.
- Core principle: "Write tests with different granularity. The more high-level you get the fewer tests you should have."

The pyramid suits backend-heavy systems with predominantly pure functions and clear unit boundaries. It also reflects the cost asymmetry: unit tests are fast and stable, E2E tests are slow and flakey, so the cheaper layer should carry more of the coverage.

**Kent C. Dodds's Testing Trophy** (https://kentcdodds.com/blog/the-testing-trophy-and-testing-classifications):

- Static (TypeScript, ESLint) at the foundation.
- Unit, then integration, then E2E ascending.
- Core principle: "The more your tests resemble the way your software is used, the more confidence they can give you."
- Relative emphasis: more weight on integration than the classic pyramid suggests.

The trophy suits frontend / integration-heavy systems where the user value lives at the integration layer (component composition, API boundaries) rather than inside individually-testable units. It is not a refutation of the pyramid; it is a re-weighting for software whose hardest bugs live at integration seams.

In practice: if your codebase is dominated by pure-function libraries, lean pyramid. If it is dominated by component-level orchestration where the integration layer is where the bugs hide, lean trophy. Most real codebases are a mix and the right distribution is empirical.
