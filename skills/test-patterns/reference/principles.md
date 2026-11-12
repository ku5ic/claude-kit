# Principles (all stacks)

## Core principles

- Test behavior, not implementation. A test that breaks when internals are refactored is too coupled.
- Aim for one clear assertion per test. Multiple related assertions are fine; sprawling ones aren't.
- Make arrange, act, assert visible in the test structure.
- Prefer fixtures over factories over repetition, and extract setup used three or more times.
- Name a test as a full sentence describing the expected behavior, not the method under test.
- A flaky test is a failure: fix or delete it, never add a retry loop.
- Each test sets up and tears down its own state and doesn't depend on order.
- Time, randomness, and network need explicit control: a fixed clock, a seed, a stub.

## Test pyramid vs testing trophy

- Martin Fowler's Test Pyramid: many fast unit tests at the base, some integration tests in the middle, few end-to-end tests at the apex. "Write tests with different granularity. The more high-level you get the fewer tests you should have."
- Kent C. Dodds's Testing Trophy: static checks (TypeScript, ESLint) at the foundation, then unit, integration, and E2E ascending, with more weight on integration than the pyramid gives it. "The more your tests resemble the way your software is used, the more confidence they can give you."
- Lean pyramid for backend-heavy code dominated by pure functions with clear unit boundaries.
- Lean trophy for frontend or integration-heavy code where the bugs hide in component composition and API boundaries.
- In a codebase that mixes both, find the distribution empirically.
