# Code-level integrity

Per function and per file, while writing or reviewing.

- A function whose name needs an "and": split it. One job per function, per `skills/kit-rules/reference/change.md` section 8 (SOLID).
- A name says what, not how: `validatePaymentBatch`, never `processItems`.
- A literal beyond `0`, `1`, `-1`, `""`, or an obvious enum gets a named constant. Add a comment only if the value's origin is non-obvious, per `skills/kit-rules/reference/change.md` section 5.
- Two stores of the same data kept in sync are a smell. Keep one source of truth, per `skills/kit-rules/reference/change.md` section 8 (DRY).
- Every error path is deliberate. A bare catch that swallows the error is a `failure`. `try/finally` without `catch` is fine when cleanup is the goal.
- Size and coupling: the numbers in [metric-thresholds.md](metric-thresholds.md), and no others.
