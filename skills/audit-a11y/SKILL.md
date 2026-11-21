---
name: audit-a11y
description: WCAG 2.2 AA audit of a component, page, or template, in any frontend stack
argument-hint: "<file path, component, or a link to an external tracker/doc>"
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
---

# /audit-a11y

WCAG 2.2 AA audit of a component, page, or template.

## Run

With no target in the arguments, ask for one first, since the auditor can't ask. Resolve a linked ticket or doc as `skills/investigate` step 0 describes, then dispatch the auditor agent (subagent_type: kit:auditor, foreground) with this file's procedure and the resolved arguments. It writes the report; reply per `skills/kit-rules/reference/output.md` section 3.

## Prerequisites

- $ARGUMENTS should specify the target (file, directory, or component name).
- Frontend surface must exist. If `<repo-context>` reports no `js` and `fd -e html -e erb -e jinja -e jinja2 -e j2 -e vue -e svelte -e astro` finds nothing: stop and say so.

## Procedure

1. Stack is in the `<repo-context>` block from the `SubagentStart` hook.
2. Confirm frontend surface exists. If not, stop.
3. Load the wcag-audit skill.
4. Read the target file(s):
   - A component: include the direct children it renders from this repo.
   - A template: include the parent template it extends or the layout that wraps it.
5. Apply the checklist from wcag-audit. Stack adaptation section inside the skill tells you which variants apply.
6. Before finalizing a finding:
   - Check whether project CLAUDE.md documents an explicit, deliberate exception for it. That is the only thing that excuses it.
   - A pattern that repeats across multiple components is not excused by repetition.
   - Flag it once against the shared source and list every consuming location; fixing the shared component resolves all of them.
   - Repetition raises priority; it does not lower it.
7. When the project already runs axe or the Storybook a11y addon in its tests, cite their results for the findings they cover and mark those `runtime-verified`. Everything else that needs a running page (such as contrast) goes under "Needs a runtime check"; never install axe or start a server.

## Not a finding

- An image the markup marks decorative (`alt=""`, `aria-hidden`) that carries no information.
- A criterion that needs a running page, such as contrast or reflow, which goes under "Needs a runtime check".
- A AAA criterion, unless the user asked for AAA.

## Severity

The wcag-audit skill's severity rubric.

## Output

Load the report-format skill and use its format. Write to the path `kit scratch-dir a11y <target-slug>` prints.

Rules specific to this audit:

- Cite WCAG criterion on every finding (e.g. 1.3.1, 4.1.2).
- Distinguish Level A, AA, AAA if audit scope is AA: AAA findings go to "Out of scope" unless the user asked for them.
- Items that need a runtime check go to the "Needs a runtime check" section. Do not fabricate verification.
- Print the report path.

## Scope

- Do not modify code during the audit. Audit only.
- Fixes are suggestions in the report, not applied changes.
