---
name: engineering-fundamentals
description: Concrete design and code-level checks for planning, review, and writing code - modularity, separation of concerns, reversibility, verifiability, function and file size, nesting, naming, magic values, error handling, requirements clarity, and verification vs validation. Complements rules/change.md, which owns the principles (minimal change, KISS, YAGNI, DRY, SOLID, comments). Use when planning or reviewing a change or design, or writing source code in any language, even if "fundamentals" is not mentioned by name.
---

# Engineering fundamentals

Concrete checks that turn `rules/change.md`'s principles into questions with observable answers. The principles themselves (minimal change, existing patterns, KISS, YAGNI, DRY, SOLID, comments, dead code) live in that rule; this skill never restates or overrides them.

| Activity                                | Apply                                   |
| --------------------------------------- | --------------------------------------- |
| Planning a change or reviewing a design | Requirements clarity, Design integrity  |
| Writing or reviewing code               | Code-level integrity, Metric thresholds |
| Reviewing whether a change is right     | Verification and validation             |

Apply only what fits. Do not pad findings to fill sections.

| File                                                                                 | Covers                                                                     |
| ------------------------------------------------------------------------------------ | -------------------------------------------------------------------------- |
| [reference/requirements-clarity.md](reference/requirements-clarity.md)               | Testable, unambiguous, complete, consistent, before non-trivial work       |
| [reference/design-integrity.md](reference/design-integrity.md)                       | Module ownership, abstraction level, separation of concerns, reversibility |
| [reference/code-level-integrity.md](reference/code-level-integrity.md)               | Per-function and per-file checks                                           |
| [reference/metric-thresholds.md](reference/metric-thresholds.md)                     | The one set of size, nesting, branch, and coupling numbers                 |
| [reference/verification-and-validation.md](reference/verification-and-validation.md) | Built it right vs built the right thing                                    |

Test design is `test-patterns`.
