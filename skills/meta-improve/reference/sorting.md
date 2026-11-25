# Sorting and deciding

## Sort

Each finding is one of four, by what the instruction did at that moment:

| Class       | What happened                                                                                                                                     |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| Misguiding  | Claude followed a rule, and the user overrode it, argued with it, or repeated an instruction it contradicts. Ceremony the user didn't want counts |
| Weak        | The right rule was loaded, and the failure happened anyway                                                                                        |
| Unreachable | The rule was never in context, or the skill's description didn't match the request                                                                |
| Uncovered   | No rule addresses it                                                                                                                              |

Weak and unreachable differ by one fact, whether the text was in context in that session. Settle it from the transcript: the skill's load, the reference's Read, or the memory files the session loaded. After a compaction, look for a reload.

## Decide

| Class       | Decision | How                                                                                                                                                                                 |
| ----------- | -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Misguiding  | Remove   | The rule, or the part that misguides, and anything that repeats it                                                                                                                  |
| Weak        | Refine   | A concrete trigger and a behavior someone could watch for, never more emphasis. Wording that history shows was tried and reverted is out                                            |
| Unreachable | Relocate | To where it's loaded when the work happens, or a description that matches how the user asks                                                                                         |
| Uncovered   | Add      | The smallest layer that works: code, a test, or a check; then an existing rule or skill; then a new one. The topic's owner, when the project keeps an owner table, decides the file |

- A rule superseded by another, or said twice, is removed whatever its class.
- A rule that was ignored is weak. Being ignored is no reason to remove it.
- A rule that never fired has no quote, so it's no finding. It may guard something that didn't come up.
- Where a rule contradicts an addition, remove that rule first, as its own change.
- One incident, taste, and a single frustrated message are no pattern.
