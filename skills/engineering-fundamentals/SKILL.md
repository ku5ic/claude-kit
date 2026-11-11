---
name: engineering-fundamentals
description: Design and code-level checks for modularity, size, nesting, naming, errors, and red-green-refactor. Use when planning, reviewing, or writing code in any language.
---

# Engineering fundamentals

Concrete checks that turn the principles in `skills/kit-rules/reference/change.md` into questions with observable answers. The principles themselves (minimal change, existing patterns, KISS, YAGNI, DRY, SOLID, comments, dead code) live in that rule; this skill never restates or overrides them.

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have. Apply only what fits, and don't pad findings to fill sections.

| Before                                                 | Read                                                                                 |
| ------------------------------------------------------ | ------------------------------------------------------------------------------------ |
| Planning non-trivial work from a request or a ticket   | [reference/requirements-clarity.md](reference/requirements-clarity.md)               |
| Planning a change or reviewing a design                | [reference/design-integrity.md](reference/design-integrity.md)                       |
| Writing or reviewing a function or a file              | [reference/code-level-integrity.md](reference/code-level-integrity.md)               |
| Judging whether a function, file, or module is too big | [reference/metric-thresholds.md](reference/metric-thresholds.md)                     |
| Restructuring code that grew, spread, or repeats       | [reference/design-patterns.md](reference/design-patterns.md)                         |
| Reviewing whether a change is right                    | [reference/verification-and-validation.md](reference/verification-and-validation.md) |
| Adding behavior or fixing a bug                        | [reference/red-green-refactor.md](reference/red-green-refactor.md)                   |

<!-- chat: skip -->

Test design is `test-patterns`.

<!-- chat: end -->
