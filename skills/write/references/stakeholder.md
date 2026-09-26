# /write stakeholder

Reframe a technical finding or proposal for a non-technical audience. Arguments: `<technical content, or path to a file with it>`.

## Procedure

1. Get the scratch directory: `!`scratch-dir.sh``.
2. Read the input. If $ARGUMENTS looks like a file path, read the file. Otherwise treat $ARGUMENTS as the content directly.
3. Identify the audience implied by the input or default to PM-level.
4. Reframe.

## Output file

Write to the path `scratch-dir.sh stakeholder <topic-slug>` prints. Print the path.

Structure:

```
# <Topic>

**Audience**: <CEO | PM | client | leadership>

## Situation

<What is going on, in plain language. No jargon unless defined.>

## Why it matters

<Business or user impact. Not the technical mechanism.>

## Options

<Only if there is a decision to make. One line per option with its tradeoff. Otherwise omit.>

## Recommendation

<What you propose, in one or two sentences, with the core reason.>
```

## Length guidance

- CEO brief: one screen, read standing up. Situation, impact, recommendation - nothing else.
- PM writeup: enough to act on without a follow-up meeting. Options and their tradeoffs survive; mechanism does not.
- Client note: as short as the CEO brief on the problem, longer on what happens next and when.

## Rules

- Replace jargon with plain language without oversimplifying to the point of being inaccurate.
- Keep tradeoffs intact. Do not hide risk or complexity, just express it in outcomes.
- Tone: direct, professional, confident. Not apologetic, not hedged.
- Do not invent context. If something in the input is ambiguous, note it as an open question rather than guessing.
- No padding. If a section has nothing meaningful, omit it.
