---
name: write-retro
description: A structured retrospective for an incident, sprint, or finished feature, its action items filed as tickets
argument-hint: "<incident timeline path, sprint label, or feature name> | <path to a prior retro>"
disable-model-invocation: true
---

# /write-retro

Structured retrospective for an incident, sprint, or finished feature. An incident's action items become tickets, and a rerun checks whether they closed and whether the failure came back.

<!-- chat: only
In claude.ai chat, the material is what the user pasted. Print the retro in the reply, and list the action items ready to paste into a tracker.
-->

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                    | Read                                                             |
| ------------------------- | ---------------------------------------------------------------- |
| Writing an incident retro | [reference/incident-practice.md](reference/incident-practice.md) |

## Procedure

1. The retro type, from the arguments:
   - Incident: something broke, users were affected, and a response happened
   - Sprint or iteration: a time-boxed period of work
   - Feature or project: a shipped deliverable
2. Read what the arguments point at. An incident's timeline file comes first: its rows fill the Timeline section, and its header gives the failure signature. With no timeline, say the timeline is reconstructed.
3. Use the matching template below.

<!-- chat: skip -->

4. Git context where it helps: `git log --since=<date>` for a sprint.

## Output file

Write to the path `kit scratch-dir retro <type>-<slug>` prints.

## Action items as tickets

Ask with AskUserQuestion whether to file the action items. On yes, run the write-tickets skill's procedure with the retro as its source, one ticket per item, through its own post step and its yes. Each ticket's URL goes under its item.

## Rerun

With a prior incident retro's path, report per action item whether its ticket closed, through the tracker, and how long it's been open. Then search the monitoring connector (`skills/kit-rules/reference/tooling.md` section 4) for the retro's failure signature since the retro's date. A match means the failure came back, with the alert or error that matched. Write to a new path, never editing the old retro.

<!-- chat: end -->

## Incident template

```markdown
# Incident retro: <short name>

Failure signature: <alert name>, <service>, <top error message with ids and numbers stripped>

## Timeline

<Minute by minute, detection through resolution. User impact at each step.>

## What happened

<Plain language summary. Root cause vs contributing factors separated.>

## What went well

<Detection speed, response coordination, tooling that helped.>

## What went poorly

<Gaps in detection, response, communication, or tooling.>

## Lucky vs systemic

<Separate "we caught it because X was awake" from "our system detected it". Lucky is not a control.>

## Action items

<Each: description, owner by role (not name), priority, type (prevent, detect, respond, contain), and its ticket once filed.>

## Followups deferred

<Good ideas that are out of scope for this retro.>
```

## Sprint or iteration template

```markdown
# Sprint retro: <sprint label>

## What shipped

<User-visible changes. Link to tickets.>

## What did not ship and why

<Planned but deferred. Distinguish scope cuts from blockers.>

## What worked

<Practices, tools, decisions.>

## What did not work

<Friction points, recurring issues, process debt.>

## Action items for next sprint

<Small, owned, time-bounded. No wishlist items.>
```

## Feature or project template

```markdown
# Retro: <feature name>

## Outcome vs goal

<Did we deliver what was planned? Where did scope shift?>

## What we learned about the problem

<Insights about users, data, constraints that we did not know at the start.>

## What we learned about the solution

<What the design got right. What we would do differently.>

## Technical debt introduced

<Explicit debt taken to ship. Note the reason and the pay-off trigger.>

## Reusable patterns

<What can be lifted into a skill, command, or team pattern.>
```

## Rules

- Action items must be concrete and owned (by role, not person). "Improve monitoring" is not an action item.
- Separate observations from interpretations. "Deploy took 40 minutes" is observation. "Deploy pipeline is slow" is interpretation.
- No blame language. Describe systems and processes, not individuals ([reference/incident-practice.md](reference/incident-practice.md)).
- Distinguish lucky from systemic.
- Keep it scannable. A retro no one reads is worse than no retro.
