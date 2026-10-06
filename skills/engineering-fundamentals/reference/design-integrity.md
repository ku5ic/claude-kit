# Design integrity

Applies when planning a change, evaluating an architecture, or reviewing a design doc. Each answer is a concrete sentence naming the module, caller, or concern, never yes/no.

- **Modularity.** Which module owns this change? If it crosses module boundaries, name them and justify.
- **Abstraction level.** Does any caller need to know an implementation detail to use the new code? If yes, the abstraction is wrong.
- **Separation of concerns.** Does any function or component combine concerns that change for different reasons (data fetching and presentation, validation and persistence)? If yes, split or justify.
- **Reversibility.** If this proves wrong after merge, what does reversing it cost? If a lot, justify it over a more reversible alternative.
- **Verifiability.** How will the result be verified? Name the test, type check, or runtime check. "Eyeball it" means the design is incomplete.
- **Deferral.** Deferring a code-level cleanup is cheap. Deferring a structural problem compounds and is rarely paid down; name it when you do.
