---
name: engineering-fundamentals
description: Engineering fundamentals reference covering requirements clarity, design integrity, code-level quality, test design, verification and validation, and metric thresholds. Use whenever the project contains source files in any compiled or scripted language (`.js`, `.ts`, `.py`, `.rb`, `.go`, `.rs`, `.java`, `.sh`, etc.), OR the user asks about code quality, design principles, SOLID, DRY, KISS, requirements clarity, test design, metrics, or any engineering review, audit, plan, or refactor task, even if "principles" or "fundamentals" are not mentioned by name.
---

# Engineering fundamentals

Foundation reference for engineering work. Loads alongside language and framework skills for any review, audit, plan, implementation, refactor, or code quality task.

Apply this checklist as the primary engineering quality reference.

## Which sections apply

| Activity                                    | Sections to apply                                         |
| ------------------------------------------- | --------------------------------------------------------- |
| Reviewing someone else's PR or diff         | Code-level integrity, Test design, Metric thresholds, V&V |
| Planning a change ad-hoc                    | Requirements clarity, Design integrity                    |
| Writing code without a plan                 | Code-level integrity                                      |
| Triaging a "this feels wrong" hunch         | Metric thresholds, V&V                                    |
| Sanity-checking docs or specs               | Requirements clarity                                      |
| Deciding extract / test / commit / refactor | Decision frameworks                                       |

Apply only what fits. Do not pad findings to fill sections.

## Reference files

| File                                                                                 | Covers                                                                                    |
| ------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------- |
| [reference/requirements-clarity.md](reference/requirements-clarity.md)               | Testable / Unambiguous / Complete / Consistent before non-trivial implementation          |
| [reference/design-integrity.md](reference/design-integrity.md)                       | Modularity, abstraction, KISS, DRY, plus SOLID principles and DRY/KISS attribution        |
| [reference/code-level-integrity.md](reference/code-level-integrity.md)               | Per-file checks plus the Stevens/Myers/Constantine cohesion and coupling hierarchies      |
| [reference/test-design.md](reference/test-design.md)                                 | Behavior, boundary, equivalence partitioning, negative cases (after Myers's testing book) |
| [reference/verification-and-validation.md](reference/verification-and-validation.md) | Boehm's V&V distinction with concrete failure modes                                       |
| [reference/metric-thresholds.md](reference/metric-thresholds.md)                     | Function size, cyclomatic complexity, McCabe < 10, SonarSource Cognitive Complexity       |
| [reference/anti-patterns.md](reference/anti-patterns.md)                             | Eight anti-patterns covering the gap between citing principles and applying them          |
| [reference/decision-frameworks.md](reference/decision-frameworks.md)                 | When to extract, add a test, commit, or refactor in place vs. defer                       |

## References

These checklists distill widely accepted software engineering practice. For deeper background:

- Modularity, abstraction, separation of concerns: Parnas (1972), "On the Criteria To Be Used in Decomposing Systems into Modules", CACM 15(12) pp 1053-1058.
- Cyclomatic complexity: McCabe (1976), "A Complexity Measure", IEEE Transactions on Software Engineering SE-2(4) pp 308-320.
- Cohesion and coupling: Stevens, Myers, Constantine (1974), "Structured design", IBM Systems Journal 13(2) pp 115-139.
- V&V distinction: Boehm (1981), "Software Engineering Economics", Prentice-Hall.
- Test design (boundary, equivalence): Myers, "The Art of Software Testing", Wiley (1979 / revised editions).
- SOLID: Robert C. Martin (2000), "Design Principles and Design Patterns" (paper). Acronym coined by Michael Feathers (~2004).
- DRY: Hunt and Thomas, "The Pragmatic Programmer" (1st ed 1999, 20th anniversary ed 2019), Addison-Wesley.
- Refactoring catalog: Martin Fowler, "Refactoring" (2nd ed, 2018).
- Construction practice: Steve McConnell, "Code Complete" (2nd ed, 2004).
- Cognitive Complexity (vendor source): https://www.sonarsource.com/resources/cognitive-complexity/
- SEI CERT Coding Standards: https://wiki.sei.cmu.edu/confluence/display/seccode

Citations are for context. Do not require Claude to read source material; the checklists above are the operational version.
