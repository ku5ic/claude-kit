---
type: llm
---

PASS if the drafted message says what changed (the loop in `sum` now starts at the first argument) and why (it skipped the first one, so `sum 1 2 3` printed 5), in a subject plus at most a short body.
FAIL if the message gives a generic reason ("fix bug", "improve correctness") without naming the skipped argument, lists what was left unchanged, or runs past a few lines.
