# Requirements clarity

Before any non-trivial implementation, the request must be:

- **Testable.** Pass or fail is observable without ambiguity. "Improve performance" with no measurable signal fails.
- **Unambiguous.** One reasonable interpretation. "Should also handle X if needed" or "be flexible" fails.
- **Complete.** Inputs, outputs, and error cases are stated or directly inferable from the codebase. A happy-path-only spec fails.
- **Consistent.** It doesn't contradict the project's `CLAUDE.md`, existing tests, or recent history.

Requirements inferred from the code instead of stated make every later change a guess. When one of the four fails, what to do is `rules/workflow.md` section 4.
