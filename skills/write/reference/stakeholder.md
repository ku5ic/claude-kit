# /write stakeholder

Reframe a technical finding or proposal for a non-technical audience. Arguments: `<technical content, or path to a file with it>`.

## Procedure

1. Read the input. If the arguments look like a file path, read the file. Otherwise treat them as the content directly.
2. Identify the audience implied by the input or default to PM-level.
3. Reframe.

## Output file

<!-- chat: skip -->

Write to the path `kit scratch-dir stakeholder <topic-slug>` prints.

<!-- chat: end -->

Plain paragraphs in this order, no headers or labels (`skills/kit-rules/reference/output.md` section 4):

```text
<Topic, one line.>

<What is going on, in plain language. No jargon unless defined.>

<Business or user impact. Not the technical mechanism.>

<Only if there is a decision to make: one line per option with its tradeoff. Otherwise omit.>

<What you propose, in one or two sentences, with the core reason.>
```

## Length guidance

- CEO brief: one screen, read standing up. Situation, impact, recommendation. Nothing else.
- PM writeup: enough to act on without a follow-up meeting. Options and their tradeoffs survive; mechanism does not.
- Client note: as short as the CEO brief on the problem, longer on what happens next and when.

## Rules

- Replace jargon with plain language without oversimplifying to the point of being inaccurate.
- Keep tradeoffs intact. Do not hide risk or complexity, just express it in outcomes.
- Tone: direct. Not apologetic, not hedged.
- Do not invent context. Ask about an ambiguity in the input.

<!-- chat: skip -->

- How to ask: `skills/kit-rules/reference/workflow.md` section 4.

<!-- chat: end -->

- No padding. If a section has nothing meaningful, omit it.
