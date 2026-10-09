# /write devnote

Explain a completed change's key reasoning, or what you found testing someone else's, in a sentence or two, developer to developer. Arguments: `<optional: which decision to focus on>`.

## Procedure

1. Look at the current diff (`git diff` / `git diff --cached`) and the conversation so far.
2. Identify the one or two decisions that were not obvious from the diff alone:
   - An assumption got checked instead of trusted.
   - A simpler option was passed over for a reason.
   - Something turned out different from what was expected going in.

   Skip anything that's just "what changed" - the diff already says that.

3. If $ARGUMENTS names a specific decision, focus on only that one.
4. If nothing non-obvious happened (the change is mechanical, or the diff already speaks for itself), say so in one line instead of padding.

## Output

Plain language, developer to developer - assume the reader is a peer engineer who doesn't need the mechanism re-explained, just the reasoning. No restating the diff, no "Summary:" preamble. Print directly to the terminal, not a file. A single decision is one or two short sentences; more than one decision gets a plain bullet per decision. No bold lead-in labels.

Write it the way you'd say it to a teammate in chat, casually. No file paths, line numbers, or source citations - name the tool or concept instead ("fuzzysort scores by the best-matching key", not `fuzzysort.js:459`). Keep the evidence, drop the receipts.

## Rules

- Say what turned out to be true and why, not what was built.
- If something was verified empirically instead of assumed, that's the interesting part - lead with it.
- Cut every sentence that's only there to sound complete.
- Voice: first person, as the developer who did the work (made the change, or tested someone else's) - "I checked X, turned out Y" - never third-person narration of the change ("This commit does X", "The change adds Y"). Must read like the developer wrote it, not like a summary of the developer's work.

## Example

A note on a teammate's PR after testing it locally. What was checked, what it showed, then one optional nit with a direction:

```
I ran it locally and everything I tried behaved as described: the redirect on /, the settings menu and the export button switch correctly with the feature flag on and off.

A nit. the `useFeatureFlags` test mock is repeated in 8 test files, maybe worth folding into the follow-up PR.
```
