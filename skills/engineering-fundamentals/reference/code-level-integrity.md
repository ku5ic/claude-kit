# Code-level integrity

## Contents

- [Per-file checks](#per-file-checks)
- [Cohesion hierarchy](#cohesion-hierarchy)
- [Coupling levels](#coupling-levels)

## Per-file checks

Applies per file, per function, during implementation or review.

- **Cohesion.** A function does one thing. If the name needs an "and", split.
- **Coupling.** A new module depends on the fewest concrete other modules possible. More than five non-stdlib imports in a new file: justify.
- **Naming.** Names describe what, not how. `processItems` is weak; `validatePaymentBatch` is concrete.
- **Magic values.** Literals beyond `0`, `1`, `-1`, `""`, and obvious enums get a named constant with a comment explaining the value.
- **Single source of truth.** Data lives in one place. Two stores being kept in sync is a smell.
- **Comments.** Explain why, not what. Remove comments that paraphrase the next line.
- **Error handling.** Each error path is intentional. Bare catches that swallow are `failure`. `try/finally` without `catch` is fine when cleanup is the goal.

## Cohesion hierarchy

Stevens, Myers, and Constantine ("Structured design", IBM Systems Journal 1974) named seven cohesion levels from worst to best. Knowing the names lets review feedback be specific instead of "low cohesion."

1. **Coincidental** -- elements grouped arbitrarily (utility module that absorbs unrelated functions). Worst.
2. **Logical** -- elements share a category but operate independently (a "string utilities" module that mixes parsing, formatting, and hashing).
3. **Temporal** -- elements run at the same time (an `init()` that mixes logging setup, DB connection, and feature-flag fetch).
4. **Procedural** -- elements share a sequence (an "order processing" module that runs validation, then save, then notify).
5. **Communicational** -- elements operate on the same data (a module of functions that all read/write a single record). Good.
6. **Sequential** -- output of one element is input to the next within the module. Good.
7. **Functional** -- the module does exactly one well-defined task. Best.

Practical use in review: rather than "this class is doing too much," cite the level: "this looks like temporal cohesion -- `init()` is grouping setup steps that change for different reasons. Consider splitting into `init_logging`, `init_db`, `init_flags`."

## Coupling levels

Same paper defines a coupling hierarchy from worst to best:

1. **Content** -- one module modifies another's internals directly. Worst.
2. **Common** -- shared global state.
3. **External** -- modules share an external imposed format (file format, protocol).
4. **Control** -- one module passes a flag that switches another's behavior.
5. **Stamp** (data-structured) -- modules pass a composite data structure where only some fields are used.
6. **Data** -- modules pass only the data they need. Good.
7. **Message** -- communication via well-defined messages with no shared state. Best.

Modern dependency-injection patterns push toward data and message coupling. Direct field access on another object's internals (content coupling) is the kind of thing static analyzers flag at the `failure` level.

## References

- Stevens, Myers, Constantine (1974), "Structured design", IBM Systems Journal 13(2) pp 115-139.
