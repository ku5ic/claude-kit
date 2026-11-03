# Rewriting a hit

A guard's deny lists each hit with its line and a fix direction. Per hit:

1. Read the whole sentence, not the matched words.
2. Ask what specific fact it points at. The number, the file, the behavior, the person, the reason.
3. Found one: write the sentence around that fact.
4. Found none: the sentence was filler. Cut it.
5. Never swap the flagged word for a synonym. "Serves as" to "acts as" keeps the tell and dodges the regexp; the next reader still sees it.

Then reread the paragraph: cutting one sentence often shows the one beside it was propping it up.

## Worked examples

| Flagged                                                            | Fact behind it              | Rewrite                                                               |
| ------------------------------------------------------------------ | --------------------------- | --------------------------------------------------------------------- |
| "The loader serves as the entry point for config."                 | It is the entry point       | "The loader is where config enters."                                  |
| "Gates rerun on each edit, ensuring consistent results."           | What reruns, and the effect | "Gates rerun on each edit, so a stale pass can't hide a new failure." |
| "Caching plays a crucial role in the gate's performance."          | How much it saves           | "Caching cuts a warm gate run from 40s to 6s."                        |
| "Refactored the parser to improve readability."                    | What changed                | "Split the parser's 300-line switch into one function per node type." |
| "Moved the check to bashguard while preserving existing behavior." | Nothing: the tests say that | "Moved the check to bashguard."                                       |
| "I hope this helps! Let me know if you need anything else."        | None                        | (cut)                                                                 |

## When the hit is wrong

A quoted example, a term of art, or a deliberate quotation: [false-positives.md](false-positives.md). Don't rewrite correct text to satisfy a regexp.
