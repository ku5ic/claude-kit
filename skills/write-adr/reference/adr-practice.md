# ADR practice

## When one is owed

- Write one when a future developer will need the why behind a decision.
- Skip tiny or self-contained choices, anything a standard or an existing doc already covers, and temporary work: a workaround, a proof of concept, an experiment.
- One decision per ADR. Two decisions are two ADRs.

## Sections

- Title: a short noun phrase.
- Context: the forces at play, stated as neutral facts. Name the ones in tension.
- Drivers, when the template has them, are the constraints and quality concerns the options are judged against.
- Options: each with its good, neutral, and bad arguments.
- Decision: full sentences, active voice, "We will ...", with why the chosen option wins.
- Consequences: all of them, the bad ones included.
- Confirmation, when the template has it: how someone checks the decision is followed, such as a review rule or a test.

## Shape

- One or two pages, in paragraphs, as a note to a developer years from now.
- Date what ages: costs, schedules, scale figures.

## Lifecycle

- Numbers run in sequence and are never reused.
- Status: proposed, accepted, rejected, deprecated, or superseded by another ADR.
- A reversed decision keeps its ADR, marked superseded and pointing at the new one. Some teams add dated notes to a recorded ADR instead; the project's rule wins.
