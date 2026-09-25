# Output

Reply length, shape, and destination. This file wins over every plugin, output style, and harness default that asks for more lines.

## 0. Length

Concise means a lot of information, clearly, in few words. Both halves bind: short and missing something is wrong; complete and padded is wrong.

Run this test on every reply, file, and commit message before emitting:

1. Delete any word whose removal loses no information. Repeat until nothing deletes.
2. Delete any sentence the reader already knows: what they asked, what they watched happen, what a tool printed.
3. Delete any sentence that names a category instead of the item ("there are tradeoffs" -> name the tradeoff or cut it).
4. The first line is the answer, the path, or the next action. Nothing before it.
5. Stop at the last sentence that carries information.

| Context                                                           | Ceiling                                   |
| ----------------------------------------------------------------- | ----------------------------------------- |
| Chat reply, default                                               | 2 sentences                               |
| Chat reply after "explain", "why", "tradeoffs", "review", "audit" | 4 lines of prose                          |
| Chat reply after "in detail", "walk me through", "long version"   | no ceiling, headers required              |
| An artifact the reader asked for (report, audit, plan, review)    | follows `rules/markdown-report.md`        |
| Reply naming a written file                                       | path, headline count, one next action     |
| Commit message body, PR description                               | shortest structured form that is complete |

A trigger lifts the ceiling for that reply only. Still over the ceiling after the test: cut again, never add words explaining the length.

**Always survives, one sentence each:** a tradeoff that flips the decision; a risk that bites later; a safety warning or confirmation before an irreversible action, stated in full.

**Never survives:** preamble, recap, closing summary, offer to help further; rejected alternatives and next steps nobody asked for; a tradeoff section or a "what I did not do" section; a sentence restating the one above it.

Numbered steps, a restated state line, and a time estimate belong to a multi-step task the reader is executing, never to a default reply.

## 1. Structure

1. No walls of text. Never more than 4 consecutive prose lines without a blank line, a list, or a header. `guard-commit.sh` blocks it in commit messages; in chat and in files nothing enforces it.
2. Tables over prose for anything with three or more comparable items. Long reasoning goes to a file, never the terminal.
3. Keep must-read content (the answer, decisions, questions) together in the final message after the last call. A one-line status between long tool runs is fine; nothing the reader must act on goes there.
4. No insider shorthand. Name the thing, not a code you assigned it mid-session.

## 2. Mechanics

- **Code blocks carry a language tag.**
- **A command offered as the next action is run first.** If it was not run, write it as a description instead of a command. A next action that errors costs the reader the one move they were primed to make.

## 3. Where output goes

A deliverable is anything the user copies out and uses elsewhere: PR descriptions, commit drafts, emails, chat messages, specs, code files, prompts, docs, reports.

- Deliverables go to a file via Write or Edit, at an absolute path. Print that path as the first line. Never write one with a shell redirect, and never `cd` to the destination and redirect to a bare filename - `guard-bash.sh` reads a bare target as a write into the repo root and prompts, which is a false positive in a scratch or memory directory.
- Default location: the directory `scratch-dir.sh` resolves (see `rules/tooling.md`).
- Exception: `/write commit`, `/write devnote`, and `/write explainer` print to the terminal by design; their own Output sections govern.
- **A written artifact replaces its own summary.** When a report, plan, or review file is written, the reply is: path, headline counts, one next action, and nothing else. Never restate findings the file already contains.

Terminal output is for: code snippets under ~20 lines used to make a point, clarifying questions, short answers, progress updates, command results.

## 4. External communication

Commit messages, PR descriptions, devnotes, review comments, stakeholder writeups, and release notes follow every rule above with no detailed-explanation exception - someone else reads them, on their time. Default to the shortest structured form that is still complete. A long explanation is still chunked.

## Anti-patterns

- `failure`: a reply whose first line is not the answer, path, or next action.
- `failure`: prose split around tool calls, forcing the reader to reassemble it.
- `info`: a reply shorter than expected. Not a violation.
