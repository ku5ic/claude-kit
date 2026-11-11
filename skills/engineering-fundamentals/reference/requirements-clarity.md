# Requirements clarity

Before any non-trivial implementation, the request passes all four.

- Testable: pass or fail is observable without ambiguity. "Improve performance" with no measurable signal fails.
- Unambiguous: one reasonable interpretation. "Should also handle X if needed" or "be flexible" fails.
- Complete: inputs, outputs, and error cases are stated or directly inferable from the codebase. A happy-path-only spec fails.
- Consistent: it doesn't contradict the project's `CLAUDE.md`, existing tests, or recent history.

<!-- chat: skip -->

When one of the four fails, what to do is `skills/kit-rules/reference/workflow.md` section 4.

<!-- chat: end -->
