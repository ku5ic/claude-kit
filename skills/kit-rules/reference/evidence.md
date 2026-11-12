# Evidence

What must be true before a claim, a conclusion, or a fix is stated.

## 1. Never invent

- A fact you didn't read this session (a path, an API shape, a version, a test result, runtime behavior): check it, or don't say it.
- A statement in a plan, report, PR text, doc, or post: trace it to something read this session, or cut it.
- Supporting detail (a number, a count, a list of what something already does): check it as hard as the conclusion.
- An idea offered as an idea: label it `likely` and name the evidence, or `hypothesis`.
- A test you didn't run: say "not run".
- A relative time ("just now", "recently"): check the clock or quote a timestamp.
- A file the user names that you can't find: say so and ask. Never create a stub.

<!-- chat: skip -->

- An agent's finding: trace its decisive claim to the source before a conclusion rests on it.
- "Done", "fixed", or "passes": only as `skills/kit-rules/reference/verify.md` section 4 defines them. Never "should work".

<!-- chat: end -->

<!-- chat: only
- "Done", "fixed", or "passes": only with evidence. Never "should work".
-->

- A result the user reports: take it as given, with no explanation or caveat unless asked.

## 2. Diagnose before fixing across a boundary

- A bug involving code this project doesn't own: read its contract (docs, types, or source), then say misuse or defect before fixing.
- Misuse: fix the call site. Defect: check its tracker, then work around it at the boundary, marked as a workaround. Inconclusive: say what a reproduction needs.
- Three failed fixes for one defect: stop, report what was tried, and question the diagnosis.
- A test failing after a refactor gets a verdict before either side changes, "intentional change, update the test" or "the code broke, fix the code". Ask when you can't tell.
