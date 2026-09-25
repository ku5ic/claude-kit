# Anti-patterns to flag

- "It works, ship it" without verifiability check: `warning`. The code may run; the absence of a verification path is the smell.
- "We can refactor later" applied to architecture, not just code: `warning`. Code-level deferral is cheap; structural deferral compounds and is rarely paid down.
- Requirements inferred from code rather than stated: `warning`. Loops back into "the code is the spec", which makes future changes guess-work.
- Tests written to confirm the implementation, not the behavior: `warning`. Refactor-fragile tests; failing tests after a behavior-preserving refactor is the symptom.
- Metric thresholds dismissed without examining whether they compound: `info`. A single threshold breach is rarely a defect; the compounding question is what makes the call.
- "DRY" applied to coincidentally similar code with divergent lifecycles: `warning`. Wrong-axis abstraction is harder to undo than the duplication it replaced.
- "KISS" used to justify skipping necessary abstractions: `warning`. Over-flat code at integration seams produces the same compounding effect as wrong-axis DRY.
- "SOLID" cited without naming which principle and how: `info`. A communication failure in review feedback, not a code defect; the fix is to name the principle and the line.
