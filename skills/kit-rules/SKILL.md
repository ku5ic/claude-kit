---
name: kit-rules
description: The kit's working rules for every reply and task. Load at the start of every session and subagent, before any other tool call.
user-invocable: false
---

# Kit rules

The hard rules, each owned by the section it names:

- The first line of a reply is the answer: `skills/kit-rules/reference/output.md` section 0.
- Never state as fact what wasn't read this session: `skills/kit-rules/reference/evidence.md` section 1.
- Never commit or push unasked: `skills/kit-rules/reference/workflow.md` section 1.
- Confirm a destructive, dependency, or config change first: `skills/kit-rules/reference/workflow.md` section 2.
- Files you write go through Write or Edit, temporary ones to `kit scratch-dir`: `skills/kit-rules/reference/output.md` section 3.

| File                                             | Covers                                             |
| ------------------------------------------------ | -------------------------------------------------- |
| [reference/output.md](reference/output.md)       | Reply length, shape, and where output goes         |
| [reference/evidence.md](reference/evidence.md)   | What must be true before a claim, a fix, or "done" |
| [reference/change.md](reference/change.md)       | How a change is sized, shaped, and styled          |
| [reference/verify.md](reference/verify.md)       | What runs before a change is done                  |
| [reference/workflow.md](reference/workflow.md)   | Git, side effects, plans, and when to ask          |
| [reference/subagents.md](reference/subagents.md) | When to spawn an agent, and checking its work      |
| [reference/tooling.md](reference/tooling.md)     | Which CLI to use, read budget, and scratch         |
