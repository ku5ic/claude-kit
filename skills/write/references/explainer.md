# /write explainer

Explain a bug, feature, module, or section of code to someone new to the codebase - full mechanism, file:line citations, developer to developer. Arguments: `<what to explain - a bug/ticket, a debug report path, a feature or module name, a file/directory, or blank to use what's already been discussed in this conversation>`.

## When to use this

The subject is already understood well enough to explain - root cause from `investigate`, a feature just built or reviewed, or a module/section of code someone needs oriented to - and the goal is understanding, not a change. Produces a citation-heavy walkthrough for a reader unfamiliar with the code. Use `/write devnote` instead for a short peer-to-peer note on a completed change. Use a direct edit or `/plan` instead when the goal is to change code, not explain it.

## Procedure

1. Establish the subject and source.
   - `$ARGUMENTS` may name a debug report or bug ticket, a feature or module name, a file or directory path, or nothing at all - in which case default to whatever's already been discussed in this conversation.
   - If a file is named, read it in full.
   - If it's a bug/ticket or feature/module name with no artifact, check the conversation and project memory for matching context before asking the user to point at one.
   - If nothing can be found, stop and ask what to explain.
2. Do not trust any existing citations - from a debug report, prior conversation, or memory - at face value. Re-read every file mentioned in its current state before repeating a citation. Code moves; a prior artifact is a starting hypothesis, not a fact.
3. Identify the natural entry point for understanding the subject: for a bug, the user action or event that starts the chain; for a feature or module, where it's mounted, its main export, or the first file a reader would open. Name where the relevant state or logic actually lives, not just where it's consumed or re-exported.
4. Trace hop by hop from the entry point through the rest of the subject. At each hop, cite file:line and state what the code does and why it's written that way - not just a restatement of what it does. Read the actual current file before citing it; never cite from memory of a source artifact alone.
5. If the subject has more than one distinct mechanism, responsibility, or (for a bug) manifestation:
   - Address each separately.
   - State explicitly why one doesn't substitute for or explain the other.

   This is usually the part a newcomer would otherwise collapse into a single story.

6. Close with the underlying design rationale or invariant tying the pieces together, if one exists. Name any known rough edge or open follow-up rather than implying the picture is fully closed.

## Output

Format:

- Structured with section headers - the point is a reader building a mental model across multiple hops, which a devnote's few sentences are too short to need broken out that far.
- Full sentences over fragments; a newcomer needs the reasoning spelled out.
- Cite every claim as `file:line`.

Where it goes:

- Default to terminal. Only write to a file when asked - honor an exact location if one is given (e.g. "on desktop").
- Otherwise use the path `scratch-dir.sh <kind> <scope-slug>` prints.

Invoking this skill lifts the `rules/output.md` section 0 ceiling, like "in detail" does - full sentences, full depth. That covers how much gets said, not how it's structured: within each header's section, still short paragraphs (no more than 4 prose lines, per rules/output.md section 1) and one hop or one point per paragraph, never one long undifferentiated block under a header.

## Rules

- Every file:line cited must have been read in this session, not carried over unverified from a stale artifact.
- State plainly when something is inferred vs. verified (e.g., "confirmed via the actual refetch call" vs. "likely, based on the naming").
- Multiple mechanisms sharing one subject - one ticket, one feature, one module - are not automatically one story. Check before merging them into a single narrative.
- Don't pad with a closing summary that restates what was just explained.
- Follows `rules/output.md` structurally even though it's exempt from the length ceiling: short paragraphs per point, headers per hop or mechanism, front-loaded conclusions before the mechanism detail that supports them.
