---
name: meta-retro
description: Retro on Claude Code sessions, one or many, what went wrong and where, every finding cited to the transcript and pointed at the kit rule or skill involved
argument-hint: "[session id or path | <N>d | <N>d all | similar] [what went wrong]"
disable-model-invocation: true
allowed-tools:
  - Bash(kit scratch-dir *)
  - Bash(kit stats *)
---

# /meta-retro

Find out with the user what went wrong in a Claude Code session, or across many, read the transcripts and the kit's logs, and report what happened with evidence. It reports and points at the kit rule or skill involved; it never changes the kit or proposes the fix.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                                      | Read                                               |
| ------------------------------------------- | -------------------------------------------------- |
| Step 2, finding and reading the transcripts | [reference/sessions.md](reference/sessions.md)     |
| Step 3, briefing the analysts               | [reference/dimensions.md](reference/dimensions.md) |

## Procedure

1. Scope. From the arguments, else ask with AskUserQuestion, this session first:
   - This session.
   - A past session, by id or path, or picked from a list of this project's recent sessions with each one's date and first prompt.
   - This project's sessions over the last N days.
   - Every project's sessions over the last N days.
   - Sessions similar to a finding, after one is confirmed (step 6).
2. Intake. Ask with AskUserQuestion, one question at a time, until a problem statement names the sessions, the turns if known, what the user expected, what happened, and what they care about (time, tokens, a repeated action, one specific action). "It took too long" is a complaint, not a statement. An already-scoped question, about one event or one turn, is its own statement. Over a window, "what keeps going wrong" is a statement. Nothing else starts before the user answers.
3. Locate per [reference/sessions.md](reference/sessions.md). One session: its transcript, its subagents' transcripts, and its lines in the kit's logs, confirmed by quoting its first prompt and timestamp, every rejected candidate named. A window: rank its sessions per the reference's Windows section, show the ranking, and ask which to read in depth, the worst three preselected. Write the case file, with the statement, the paths, line and byte counts, and the record meanings found, to the report's sibling folder.
4. Read the region around the reported problem yourself. Then dispatch one analyst per dimension in [reference/dimensions.md](reference/dimensions.md), in parallel, each a general-purpose agent with model `sonnet`, given the case file's absolute path, the context-safety rules, and its dimension's brief. Several sessions or a long transcript get split, one analyst per dimension per session or turn range. A finding returned without `path:line` is dropped.
5. Point each kept finding at the kit's owner for it: the kit-rules reference section, the skill, or the hook rule (by name, from `guards.jsonl`) that governs the behavior. Nothing in the kit governs it, say so.
6. Similar sessions, when the scope asked for them or the user does after the report. Turn a confirmed finding into a signature per the reference's Similar section, find the candidate sessions, and dispatch one analyst per candidate to confirm or reject the match. The matches go in their own section.

## Report

Load the report-format skill and use its format, written to the path `kit scratch-dir session <session-id-prefix, or the window like 14d>` prints, with these narrowings:

- The TL;DR's first bullet is the verdict on the problem statement, with its confidence (high, medium, low) and what would raise it.
- A Timeline table, one row per prompt the user typed, with its line, time, a one-line request, and the events in that turn (skills, subagents, compactions, guard blocks, errors). Over a window, a Sessions table instead, one row per session with its date, first prompt, rank, and whether it was read in depth.
- Over a window, a Patterns section first: each finding that recurs across sessions, with its count and the sessions it came from.
- One section per dimension, each finding as a sentence, its `path:line` and a quote of at most 200 characters, its turns, its confidence, and its kit owner. A dimension with nothing found says what it checked.

## Reply

The report path and the verdict line.

## Stop

- Read-only. A transcript or log is never edited, moved, or deleted.
- Transcripts hold the user's private data. The report stays in scratch, and nothing is posted or exported.
- No fix and no advice on changing the kit. Findings name the owner, and the user decides.
- Only what the user typed is the user's words. Hook output, system reminders, and tool results aren't, and in a subagent's transcript "user" is the parent agent.

Arguments: `$ARGUMENTS`
