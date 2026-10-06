# Code-level integrity

Applies per function and per file, while writing or reviewing.

- **One job per function.** If its name needs an "and", split it.
- **Naming.** Names say what, not how. `processItems` is weak; `validatePaymentBatch` is concrete.
- **Magic values.** A literal beyond `0`, `1`, `-1`, `""`, or an obvious enum gets a named constant. The name explains it; add a comment only if the value's origin is non-obvious, per `rules/change.md` section 5.
- **Single source of truth.** Data lives in one place. Two stores kept in sync is a smell.
- **Error handling.** Every error path is deliberate. A bare catch that swallows the error is a `failure`. `try/finally` without `catch` is fine when cleanup is the goal.
- **Size and coupling.** Use the numbers in [metric-thresholds.md](metric-thresholds.md); they are the only ones.
