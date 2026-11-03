---
type: llm
focus: { source: file, path: README.md }
---

PASS if the README names what the project has (a `sum` function in calc.sh and a test in test.sh), shows how to run the test, and every sentence states a fact a reader could check against the code.
FAIL if it praises the project, promises features the code doesn't have, pads with sections that hold nothing specific (a generic "Contributing" or "License" with no content behind it), or reads as a template with the names swapped in.
