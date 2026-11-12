---
name: kit-rules
description: The kit's working rules for every reply and task. Load at the start of every session and subagent, before any other tool call.
user-invocable: false
---

# Kit rules

The hard rules, each owned by the section it names:

- The first line of a reply is the answer: `skills/kit-rules/reference/output.md` section 0.
- A fact not read this session gets checked, or goes unsaid: `skills/kit-rules/reference/evidence.md` section 1.
- Never commit or push unasked: `skills/kit-rules/reference/workflow.md` section 1.
- Confirm a destructive, dependency, or config change first: `skills/kit-rules/reference/workflow.md` section 2.
- Temporary files go to `kit scratch-dir`: `skills/kit-rules/reference/output.md` section 3.

Read every file in this table as soon as this skill loads. The rules live in those files, and this one only points at them.

| Before                                                             | Read                                                     |
| ------------------------------------------------------------------ | -------------------------------------------------------- |
| The first edit of a task, or acting on a request with two readings | [reference/workflow.md](reference/workflow.md) section 4 |
| A commit, push, branch, install, delete, or plan                   | [reference/workflow.md](reference/workflow.md)           |
| Writing a file, a report, or a message someone else reads          | [reference/output.md](reference/output.md)               |
| Editing code                                                       | [reference/change.md](reference/change.md)               |
| A claim about code, a version, or a result                         | [reference/evidence.md](reference/evidence.md)           |
| Saying done, fixed, or passes                                      | [reference/verify.md](reference/verify.md)               |
| Spawning an agent                                                  | [reference/subagents.md](reference/subagents.md)         |
| A search, a read over several files, or a scratch path             | [reference/tooling.md](reference/tooling.md)             |
