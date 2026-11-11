# Chat replies and subagent reports

Length and shape are `skills/kit-rules/reference/output.md` section 0; this covers the phrases.

<!-- chat: skip -->

A chat reply can't be blocked: Claude Code shows it before any hook can see it. The Stop hook scans it and the next prompt carries a correction. A subagent's report is scanned before the parent reads it and sent back once.

<!-- chat: end -->

| Tell                                                               | Rule                 | Instead                                   |
| ------------------------------------------------------------------ | -------------------- | ----------------------------------------- |
| Stock openers: "Certainly!", "Great question"                      | `stock-opener`       | The answer as the first line              |
| Closing offers: "Let me know if you ...", "Happy to help"          | `offer-more`         | Stop at the last sentence that informs    |
| "I hope this helps"                                                | `hope-helps`         | Cut                                       |
| "You're absolutely right"                                          | `absolutely-right`   | What changes because they're right        |
| "In conclusion" and a recap                                        | `in-conclusion`      | Cut; the reader just read it              |
| Corrective framing: "should not be confused with"                  | `corrective-framing` | Say what it is                            |
| Clerk tone: "I've carefully reviewed", apologies for inconvenience | none, judgment       | What was checked and what it showed       |
| Hedging disclaimers about what isn't documented, then a guess      | none, judgment       | "Not documented; my guess is X because Y" |

Honest uncertainty isn't a tell. "Not run", "not sure, likely X" stay: `skills/kit-rules/reference/evidence.md` section 1.
