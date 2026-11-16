---
name: write-tickets
description: Tickets from a plan, a report, or a description, posted to the project's tracker after the user's yes
argument-hint: "<plan or report path, a description, or a ticket or doc URL>"
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
  - Bash(kit scratch-dir *)
---

# /write-tickets

Turn a plan, a report's findings, or a description into tickets someone can pick up and check, then post them to the project's tracker once the user says yes.

Tone and shape: `skills/kit-rules/reference/output.md` section 4, which wins over this file.

<!-- chat: only
In claude.ai chat, the source is what the user pasted, and nothing posts. Print the tickets in the reply, one block each, ready to paste.
-->

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                       | Read                                                         |
| ---------------------------- | ------------------------------------------------------------ |
| Step 2, cutting the work     | [reference/splitting.md](reference/splitting.md)             |
| Step 3, drafting each ticket | [reference/ticket-practice.md](reference/ticket-practice.md) |
| Step 3, drafting a bug       | [reference/bug-tickets.md](reference/bug-tickets.md)         |

## Procedure

<!-- chat: skip -->

A linked ticket or doc is resolved as `skills/investigate` step 0 describes. The tracker is found per `skills/kit-rules/reference/tooling.md` section 4, and the code host's issues count as one. Its template comes first, by the code-host-patterns skill's "Issue template" row or the connector's.

<!-- chat: end -->

1. Source. The plan's steps, the report's findings, or the description, read whole.
2. Cut the work per [reference/splitting.md](reference/splitting.md). A report's findings that one change fixes become one ticket.
3. Draft each ticket per [reference/ticket-practice.md](reference/ticket-practice.md), in the tracker's template when it has one. Each carries its source, the plan step or the finding's severity and `file:line`.

<!-- chat: skip -->

4. Labels, milestones, projects, and assignees are only ones the tracker already has, read first. None get created.

## Output file

The drafts go to the path `kit scratch-dir tickets <source-slug>` prints, one section per ticket, the parent first when there is one.

## Post (after the user's yes)

1. Ask with AskUserQuestion whether to post, naming the tracker and the ticket count.
2. Post one ticket per call, the parent first. On the code host, by the code-host-patterns skill's "Issue, create" row, paced by its limits. Otherwise through the connector's create tool. Sub-tickets link to the parent.
3. Log each ticket's URL or id under its section in the drafts file.
4. A failure stops the run. Report which posted and which didn't, since a blind retry can post a duplicate.

## Reply

The drafts path and the ticket count, then each posted ticket's URL.

<!-- chat: end -->

<!-- chat: skip -->

Arguments: `$ARGUMENTS`

<!-- chat: end -->
