---
name: investigate
description: Read-only investigation. Use when the user asks how, why, or where something works in the code, or reports unexpected behavior without asking for a change. Ends at findings, never edits.
argument-hint: <a question, a symptom, or a link to an external ticket/doc>
---

## When to use this

- A question: how, why, or where something works, what a module does, which approach fits.
- A symptom: unexpected behavior without a clear failing signal to fix directly.

Not for a request to change code. That's a plan or a direct edit.

<!-- chat: skip -->

A plan goes through plan mode (`/plan`).

<!-- chat: end -->

## Procedure

<!-- chat: skip -->

0. Resolve external context. If the arguments contain a URL with little inline description, resolve it before anything else: identify the service from the domain, find its fetch tool with ToolSearch (`app.clickup.com` -> ClickUp tools, `notion.so` -> Notion, `github.com` -> `gh` or the GitHub tools), call it, and take the scope and requirements from what comes back. The resolved text becomes the effective arguments. Never hand a bare link to a subagent.

<!-- chat: end -->

1. Classify the input as a question or a symptom. For a symptom, confirm observed behavior, expected behavior, entry point, and known repro steps; if observed vs expected is still missing, ask before continuing.

### Question

<!-- chat: skip -->

2. Skim: project CLAUDE.md, `rg` for the names involved, read the matched sections. Answer from what was read, citing `file:line`, each claim checked per `skills/kit-rules/reference/evidence.md` section 1.
3. A plan step that changes existing code opens with the three gatherer questions below, sent to `kit:gatherer` in one message. Trace each report's decisive fact before building on it (`skills/kit-rules/reference/subagents.md` section 2).
4. When the question compares approaches, one of them is the cheapest option, per `skills/kit-rules/reference/change.md` section 7.

### Gatherer questions

Each is the whole prompt for one `kit:gatherer` run, with the paths or symbols at stake filled in.

1. Precedent: how does this project already do <the kind of change>? One instance per pattern found.
2. Tests: which tests reference <the code about to change>, and what does each assert?
3. History: what changed <the lines involved> and when, by `git log -L` and `git blame`: commit, date, subject.

<!-- chat: end -->

<!-- chat: only
In claude.ai chat, the code the user pasted is all you have. Answer from it, citing the file and line where the paste shows them, each claim checked against the paste per `skills/kit-rules/reference/evidence.md` section 1. If the answer turns on code that isn't pasted, say which and ask for it. When the question compares approaches, one of them is the cheapest option, per `skills/kit-rules/reference/change.md` section 7.
-->

### Symptom

<!-- chat: skip -->

2. Delegate fault localization to the debugger agent (subagent_type: kit:debugger, foreground). It has no MCP tools, so its prompt carries fully resolved text, never a link. It follows:
   - Reproduce with the narrowest command; 3 attempts before calling it non-deterministic.
   - `git log -10 --oneline -- <affected paths>`; a commit aligned with onset is the prime suspect.
   - Trace from the entry point to where observed diverges from expected; stop at library boundaries; cap at 10 files.
   - State one hypothesis ("caused by X in file Y at line Z") before checking it.
   - Probe least-invasive first: read closer, run an existing test, `git bisect`, then one reverted log line or assertion.
   - Max 3 hypothesis cycles; if exhausted, report what was ruled out.

<!-- chat: end -->

<!-- chat: only
In claude.ai chat, nothing can be run, so read instead. Trace from the entry point to where observed diverges from expected, and stop at library boundaries. State one hypothesis ("caused by X in file Y at line Z") before checking it against the paste. Ask for logs or a missing file instead of guessing at them. After 3 hypothesis cycles, report what was ruled out.
-->

3. Across a dependency boundary, name misuse vs defect per `skills/kit-rules/reference/evidence.md` section 3.

## Output

- Short answer: inline, first line is the answer.

<!-- chat: skip -->

- Symptom, or a question that needed gatherers: a report at the path `kit scratch-dir debug <scope-slug>` or `kit scratch-dir explore <scope-slug>` prints, then the reply `skills/kit-rules/reference/output.md` section 3 describes, its headline the root cause or answer in one sentence.

<!-- chat: end -->

<!-- chat: only
In claude.ai chat, a symptom gets the same sections in the reply, with no report file: the root cause or answer first, then the findings, each hypothesis and its outcome, and the next step.
-->

<!-- chat: skip -->

```markdown
# <Debug|Explore>: <one-line description>

Generated: <ISO timestamp>
Scope: <entry point or area>

## Observed vs expected (symptom) / Question

## Findings

<file:line citations, each a fact read this session>

## Hypotheses (symptom)

1. <hypothesis> -> <confirmed / ruled out / inconclusive>

## Root cause / Answer

## Next step

<a one-line fix to make directly, or "plan it: <scope>">
```

Questions the requester can answer: `skills/kit-rules/reference/workflow.md` section 4.

## Plan mode

Outside plan mode, stop at findings. In plan mode, the findings feed the plan file instead of a separate report.

<!-- chat: end -->

## Stop

- Never edit source.

<!-- chat: skip -->

- The one exception is the debugger's single reverted probe.

<!-- chat: end -->

- If scope grows beyond the question or entry point, surface it and ask.
- If the root cause is in a dependency or external system, document the boundary and stop.
