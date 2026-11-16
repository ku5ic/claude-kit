# Analyst dimensions

Every analyst reads the case file first, follows its context-safety rules, uses only the paths it names, and returns findings in one shape: a sentence, an absolute `path:line` with a quote of at most 200 characters, the turns, and a confidence. Then one line saying what it checked. It changes nothing and says nothing about what the kit should change.

## Plan adherence

- Recover the agreed plan: a plan file the session wrote or read (`.claude/plans/`), or a numbered plan the user said yes to. Quote each step with its `path:line`.
- Map each step to the calls that carried it out. Report steps skipped, done out of order, changed without saying so, or work no step covers.
- Mark compactions and resumes. Drift right after one is its own finding.
- In a plan with a step pause, report each step that ran past its pause without the user's word.

## Repeated work

- List each tool call as its tool and key (the file for reads and edits, the command for shell, the description for a subagent).
- Report reads and searches repeated 3 or more times, and edits, commands, and subagent briefs repeated twice, except status checks and test runs.
- For each repeat, say what came between: an edit (not a finding), a compaction (a finding on the compaction), or nothing (a finding).
- A conclusion the assistant reached twice is repeated work. Quote both.

## Stumbles

- Tool errors, failed commands, the same call retried after an error, edits reverted, the assistant backtracking in its own text, and the user correcting the last action.
- Guard blocks from `guards.jsonl` for this session, each with its rule and what the assistant did next.
- For each, what happened next: recovered in the turn, recovered later at a named line, or never. Identical failures are one finding with a count.

## Rule fires

- Every `guards.jsonl` line for this session, grouped by rule, with its count and the turns it fired in.
- Hook lines injected into the transcript that name a rule or a ceiling (a reply over its word ceiling, a required skill), and whether the next turn followed them.
- A rule that fired many times in one turn is a loop. Report the first and last line.

## Skill timeline

- The skills activated per turn, from `skills.jsonl` and the transcript's Skill calls.
- The skills a hook marked required for the turn, and whether they loaded before the first other tool call.
- A turn whose request matches a skill's description with no activation, quoting the request and the description. Report the match, not whether it was wrong.

## Quality evidence

- Every test or check run with its result, and what followed a failure.
- Each claim of done, fixed, passing, or verified, with the tool result that backs it in the same turn, or none.
- Each commit's message against the calls before it, and the plan's committed work never committed.
- Review findings raised and what happened to each.

## Cost and time

- Turn durations from `turn_duration` records. The five longest, and gaps of more than ten minutes between records, saying what the gap was only when the records show it.
- Token use only from usage fields the case file recorded, saying whether each counter is per call or cumulative.
- The ten largest tool results, with their tool and turn.
- Each compaction, with what the session was doing when it fired.
