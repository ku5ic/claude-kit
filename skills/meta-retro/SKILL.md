---
description: Structured retrospective for an incident, sprint, or completed feature
argument-hint: "<context: incident summary, sprint label, or feature name>"
model: haiku
disable-model-invocation: true
context: fork
---

## Procedure

1. Get the scratch directory: `!`scratch-dir.sh``.
2. Identify the retro type from $ARGUMENTS:
   - Incident: something broke, users affected, response happened
   - Sprint or iteration: time-boxed period of work
   - Feature or project: a shipped deliverable
3. Read $ARGUMENTS. If it references a file (timeline, postmortem draft, sprint notes): read it.
4. Pull git context if useful: `git log --since=<date>` for sprint retros.
5. Use the matching template below.

## Output file

Write to `$(scratch-dir.sh)/retro-<type>-<slug>-<YYYYMMDD-HHMM>.md`. Print the path.

## Incident template

```
# Incident retro: <short name>

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

<Each: description, owner by role (not name), priority, type (prevent, detect, respond, contain).>

## Followups deferred

<Good ideas that are out of scope for this retro.>
```

## Sprint or iteration template

```
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

```
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
- No blame language. Describe systems and processes, not individuals.
- Distinguish lucky from systemic.
- Keep it scannable. A retro no one reads is worse than no retro.
