---
name: meta-improve
description: Turn retro findings into proposed changes to rules, skills, and hooks, each backed by quotes from two sessions
argument-hint: "[retro report path(s) | <N>d | <N>d all]"
allowed-tools:
  - Bash(kit scratch-dir *)
  - Bash(kit stats *)
  - Bash(wc -w *)
---

# /meta-improve

Read retro reports, sort what they found by what the instructions did, and propose the changes the evidence supports. `/meta-retro` finds what happened and proposes nothing. `/meta-improve` proposes changes and makes none.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                          | Read                                                      |
| ------------------------------- | --------------------------------------------------------- |
| Step 4, sorting the findings    | [reference/sorting.md](reference/sorting.md)              |
| Step 6, writing each proposal   | [reference/proposals.md](reference/proposals.md)          |
| Reading a transcript line again | `skills/meta-retro/reference/sessions.md`, Context safety |

## Procedure

1. Reports. From the arguments, else this project's last 14 days:
   - Paths: those reports.
   - `<N>d`: the `session-*.md` files in `kit scratch-dir` written in the last N days. `<N>d all`: the same in every directory `logs/scratch-registry.txt` under the Claude config dir lists.
   - None there: run the procedure in `skills/meta-retro/SKILL.md` over that window, its questions included, then go on from its report.
2. Earlier rounds. Read every `improve-*.md` in the same directories, for what was rejected and which follow-ups are due today or before.
3. Check each finding. Read its transcript line and compare the quote. A quote that isn't there drops the finding, listed under Rejected. Then read the turn before it, since the assistant's move is half the evidence.
4. Sort each kept finding per [reference/sorting.md](reference/sorting.md), after reading the rule, skill, or hook the retro pointed at, and its `git log`. Group findings by the one change that would prevent them. Once the groups are named, read every finding against every group again, since a group named late undercounts. A session counts once per group.
5. Decide per group, in this order, and stop at the first that fits: remove, refine, relocate, add. "No change" is a result.
6. Write a proposal for each group that meets the bar in [reference/proposals.md](reference/proposals.md). A group that doesn't goes under Rejected.
7. Follow-ups due. For each, read the sessions since the proposal's date with the filter it names, and report three things: whether the behavior came back, how many sessions could have shown it, and whether the user corrected the same thing another way. Zero sessions that could have shown it establishes nothing, and the report says so.

## Report

To the path `kit scratch-dir improve <window or slug>` prints, per the report-format skill's Readability section and Rules:

- TL;DR: how many proposals of each kind, the one to decide first, and the window with its session and report counts.
- Proposals, most sessions first, in the shape of [reference/proposals.md](reference/proposals.md).
- Follow-ups due, from step 7.
- Rejected, one line each: the change considered, its quotes as `path:line`, its session count, and what it failed.

## Reply

The report path, the counts, and the proposal to decide first.

## Stop

- Nothing is edited, and no retro report, transcript, or log is changed. The user picks proposals, and a picked one is a change like any other (`skills/kit-rules/reference/workflow.md` section 4).
- Transcripts hold private data. The report stays in scratch, and a quote from another project is cut to what the finding needs.
- Only what the user typed is the user's words (`skills/meta-retro/SKILL.md`, Stop).

Arguments: `$ARGUMENTS`
