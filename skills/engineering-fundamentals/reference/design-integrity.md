# Design integrity

## Contents

- [Core checks](#core-checks)
- [SOLID principles](#solid-principles)
- [DRY and KISS attributions](#dry-and-kiss-attributions)

## Core checks

Applies when planning a change, evaluating an architecture, or reviewing a design doc. For each, the answer must be a concrete sentence, not yes/no.

- **Modularity.** Which module owns this change? If the change crosses module boundaries, name them and justify.
- **Abstraction level.** Does any caller need to know an implementation detail to use the new code? If yes, abstraction is wrong.
- **Separation of concerns.** Does any function or component combine independently changing concerns (data fetching + presentation, validation + persistence)? If yes, split or justify.
- **KISS.** Is the simplest sufficient solution chosen? Name any non-obvious complexity and why it earns its place.
- **DRY judgment.** If duplication exists, is it along a stable axis or a divergent one? Duplication with divergent lifecycles is correct, not a violation.
- **Reversibility.** If this proves wrong after merge, what is the cost to reverse? If high, justify the choice over a more reversible alternative.
- **Verifiability.** How will the implemented code be verified? Name the test, type check, or manual check. "Manual eyeball" means the design is incomplete.

## SOLID principles

Articulated by Robert C. Martin in his 2000 paper "Design Principles and Design Patterns"; the SOLID acronym was coined around 2004 by Michael Feathers. The five principles, with the canonical one-line summary for each:

- **S - Single Responsibility Principle (SRP).** "There should never be more than one reason for a class to change." A class with two reasons to change is two classes glued together.
- **O - Open-Closed Principle (OCP).** "Software entities should be open for extension, but closed for modification." Add new behavior by adding code, not by editing existing code.
- **L - Liskov Substitution Principle (LSP).** Subtypes must be substitutable for their base types without breaking the client. A subclass that violates the base contract is a design smell, not a polymorphism win.
- **I - Interface Segregation Principle (ISP).** "Clients should not be forced to depend upon interface methods that they do not use." Many small interfaces beat one large interface that consumers only partially implement.
- **D - Dependency Inversion Principle (DIP).** "Depend upon abstractions, not concretes." High-level modules should not depend on low-level modules; both should depend on interfaces.

In review, cite the specific principle the diff violates ("the new method on `OrderService` mixes the order-validation concern with the email-sending concern; SRP suggests splitting"). Citing "SOLID" without naming which principle is meaningless feedback.

## DRY and KISS attributions

- **DRY (Don't Repeat Yourself)** is from Hunt and Thomas, "The Pragmatic Programmer" (Addison-Wesley, 1st ed 1999, 20th anniversary ed 2019). The book frames DRY as duplication of _knowledge_, not just duplication of _code_: two pieces of code that happen to look the same but encode different decisions are not DRY violations.
- **KISS (Keep It Simple, Stupid)** is widely attributed to Kelly Johnson (Lockheed, 1960s). The software-engineering use of the term has no single canonical source; it has been folded into broader minimalism guidance under various names (YAGNI, "do the simplest thing that could possibly work", etc.). The skill cites KISS by name as a heuristic; it does not claim a software-specific origin.

## References

- Parnas (1972), "On the Criteria To Be Used in Decomposing Systems into Modules", CACM 15(12) pp 1053-1058.
- Robert C. Martin (2000), "Design Principles and Design Patterns" (paper).
- Hunt and Thomas (1999, 2019), "The Pragmatic Programmer".
