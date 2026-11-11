---
name: engineering-fundamentals
description: Design and code-level checks for modularity, size, nesting, naming, errors, and red-green-refactor. Use when planning, reviewing, or writing code in any language.
---

# Engineering fundamentals

Concrete checks that turn the principles in `skills/kit-rules/reference/change.md` into questions with observable answers. The principles themselves (minimal change, existing patterns, KISS, YAGNI, DRY, SOLID, comments, dead code) live in that rule; this skill never restates or overrides them.

| Activity                                | Apply                                                    |
| --------------------------------------- | -------------------------------------------------------- |
| Planning a change or reviewing a design | Requirements clarity, Design integrity                   |
| Writing or reviewing code               | Code-level integrity, Metric thresholds, Design patterns |
| Adding behavior or fixing a bug         | Red-green-refactor                                       |
| Reviewing whether a change is right     | Verification and validation                              |

Apply only what fits. Do not pad findings to fill sections.

| File                                                                                 | Covers                                                                     |
| ------------------------------------------------------------------------------------ | -------------------------------------------------------------------------- |
| [reference/requirements-clarity.md](reference/requirements-clarity.md)               | Testable, unambiguous, complete, consistent, before non-trivial work       |
| [reference/design-integrity.md](reference/design-integrity.md)                       | Module ownership, abstraction level, separation of concerns, reversibility |
| [reference/code-level-integrity.md](reference/code-level-integrity.md)               | Per-function and per-file checks                                           |
| [reference/metric-thresholds.md](reference/metric-thresholds.md)                     | The one set of size, nesting, branch, and coupling numbers                 |
| [reference/design-patterns.md](reference/design-patterns.md)                         | Which pattern a size, spread, or repetition signal calls for               |
| [reference/verification-and-validation.md](reference/verification-and-validation.md) | Built it right vs built the right thing                                    |
| [reference/red-green-refactor.md](reference/red-green-refactor.md)                   | The test-first cycle for new behavior and bug fixes                        |

<!-- chat: skip -->

Test design is `test-patterns`.

<!-- chat: end -->
