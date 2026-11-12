# Design integrity

When planning a change, evaluating an architecture, or reviewing a design doc. Answer each with a concrete sentence naming the module, caller, or concern, never yes or no.

- Modularity: name the module that owns the change. A change that crosses module boundaries names them and justifies the crossing.
- Abstraction level: a caller that needs an implementation detail to use the new code means the abstraction is wrong.
- Separation of concerns: a function or component that combines concerns changing for different reasons (data fetching and presentation, validation and persistence) gets split or justified.
- Reversibility: name the cost of reversing the change after merge. A costly one is justified over a more reversible alternative.
- Verifiability: name the test, type check, or runtime check that verifies the result. "Eyeball it" means the design is incomplete.
- Deferral: deferring a code-level cleanup is cheap. A deferred structural problem compounds and is rarely paid down. Name it when you defer one.
