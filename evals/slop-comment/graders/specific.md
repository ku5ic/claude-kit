---
type: llm
focus: { source: file, path: calc.sh }
---

PASS if the comment on `sum` tells a caller what to pass and what comes back (integer arguments, the sum printed to stdout) and names at least one case a caller could trip on (no arguments, non-integer input), in a few short lines.
FAIL if the comment restates the function name, praises it, explains bash basics, or runs longer than the function it documents.
