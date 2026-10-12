# /write stakeholder

Reframe a technical finding or proposal for a non-technical audience. Arguments: `<technical content, or path to a file with it>`.

## Procedure

1. Read the input. If $ARGUMENTS looks like a file path, read the file. Otherwise treat $ARGUMENTS as the content directly.
2. Identify the audience implied by the input or default to PM-level.
3. Reframe.

## Output file

Write to the path `kit scratch-dir stakeholder <topic-slug>` prints.

Plain paragraphs in this order, no headers or labels (`rules/output.md` section 4):

```text
<Topic, one line.>

<What is going on, in plain language. No jargon unless defined.>

<Business or user impact. Not the technical mechanism.>

<Only if there is a decision to make: one line per option with its tradeoff. Otherwise omit.>

<What you propose, in one or two sentences, with the core reason.>
```

## Length guidance

- CEO brief: one screen, read standing up. Situation, impact, recommendation - nothing else.
- PM writeup: enough to act on without a follow-up meeting. Options and their tradeoffs survive; mechanism does not.
- Client note: as short as the CEO brief on the problem, longer on what happens next and when.

## Rules

- Replace jargon with plain language without oversimplifying to the point of being inaccurate.
- Keep tradeoffs intact. Do not hide risk or complexity, just express it in outcomes.
- Tone: direct. Not apologetic, not hedged.
- Do not invent context. Ask about an ambiguity in the input per `rules/workflow.md` section 4 rather than guessing.
- No padding. If a section has nothing meaningful, omit it.
