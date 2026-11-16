# Review scope

- A finding is about design fit, behavior and edge cases, needless complexity, tests that wouldn't fail if the code broke, unclear names, comments that restate the code, missed docs, or a break from the project's style and nearby code.
- Weigh a finding by the code's health: does the change leave the codebase better than before? Don't hold it for polish.
- A point resting on taste, not a fact or a rule, is a nit or is dropped.
- Read past the diff. The surrounding file and the callers decide whether a line is wrong.
- The summary names what the review didn't cover, such as files skipped or behavior not run.
