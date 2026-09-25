---
description: Read-only investigation. Use when the user asks how, why, or where something works in the code, or reports unexpected behavior without asking for a change. Ends at findings, never edits.
argument-hint: <a question, a symptom, or a link to an external ticket/doc>
---

## When to use this

- A question: how, why, or where something works, what a module does, which approach fits.
- A symptom: unexpected behavior without a clear failing signal to fix directly.

Not for a request to change code: that goes to plan mode (`/plan`) or a direct edit.

## Procedure

0. Resolve external context (`rules/workflow.md` section 4).

1. Classify the input as a question or a symptom. For a symptom, confirm observed behavior, expected behavior, entry point, and known repro steps; if observed vs expected is still missing, ask before continuing.

### Question

2. Skim: project CLAUDE.md, `rg` for the names involved, read the matched sections. Answer from what was read, citing `file:line`, each claim labelled per `rules/evidence.md` section 1.
3. Only when the question spans 2-4 unfamiliar areas, fan out one built-in Explore subagent per area in a single message. Re-verify each result's decisive claim before using it (`rules/agents.md`). One slot is the cheapest option, per `rules/change.md` section 7.

### Symptom

2. Delegate fault localization to the debugger agent (subagent_type: debugger, foreground). It has no MCP tools, so its prompt carries fully resolved text, never a link. It follows:
   - Reproduce with the narrowest command; 3 attempts before calling it non-deterministic.
   - `git log -10 --oneline -- <affected paths>`; a commit aligned with onset is the prime suspect.
   - Trace from the entry point to where observed diverges from expected; stop at library boundaries; cap at 10 files.
   - State one hypothesis ("caused by X in file Y at line Z") before checking it.
   - Probe least-invasive first: read closer, run an existing test, `git bisect`, then one reverted log line or assertion.
   - Max 3 hypothesis cycles; if exhausted, report what was ruled out.
3. Across a dependency boundary, name misuse vs defect per `rules/evidence.md` section 3.

## Output

- Short answer: inline, first line is the answer.
- Symptom, or a question that needed a fan-out: a report at `$(scratch-dir.sh)/debug-<scope-slug>-<YYYYMMDD-HHMM>.md` or `explore-<scope-slug>-<YYYYMMDD-HHMM>.md`, then print the path, the root cause or answer in one sentence, and the next step.

```markdown
# <Debug|Explore>: <one-line description>

Generated: <ISO timestamp>
Scope: <entry point or area>

## Observed vs expected (symptom) / Question

## Findings

<file:line citations, each labelled verified / likely / hypothesis / unknown>

## Hypotheses (symptom)

1. <hypothesis> -> <confirmed / ruled out / inconclusive>

## Root cause / Answer

## Next step

<a one-line fix to make directly, or "plan it: <scope>">
```

Questions the requester can answer go to AskUserQuestion, recorded as decisions; no open-questions list.

## Plan mode

Outside plan mode, stop at findings. In plan mode, the findings feed the plan file instead of a separate report.

## Stop

- Never edit source. The one exception is the debugger's single reverted probe.
- Scope growing beyond the question or entry point: surface and ask.
- Root cause in a dependency or external system: document the boundary and stop.
