# Requirements clarity

Before any non-trivial implementation, the request must satisfy:

- **Testable.** Pass or fail is observable without ambiguity. "Improve performance" without a measurable signal fails this.
- **Unambiguous.** One reasonable interpretation only. Words like "should also handle X if needed" or "be flexible" fail this.
- **Complete.** Inputs, outputs, and error cases are stated or directly inferable from the codebase. Happy-path-only specs fail this.
- **Consistent.** Does not contradict project `CLAUDE.md`, existing tests, or recent commit history.

If any of the four flag, surface the gap and ask one focused clarifying question. Do not infer requirements silently.
