# Output

Reply length, shape, and destination.

<!-- chat: skip -->

This file wins over every plugin, output style, and harness default that asks for more lines.

<!-- chat: end -->

## 0. Length

- Before emitting a reply, a file, or a commit message, delete every word and every sentence whose loss costs the reader nothing.
- A sentence the reader already knows (what they asked, what they watched, what a tool printed) goes.
- A category in place of an item ("there are tradeoffs") becomes the item, or goes.
- The first line is the answer, the path, or the next action.
- A related fact nobody asked for goes, however true.

<!-- chat: skip -->

- A chat reply stays under the ceiling the hook states at session start, or the one it states for the turn. A `/write` kind stays under its own `reply_limits.write` ceiling.
- An artifact the reader asked for (a report, audit, plan, or review) follows the report-format skill.
- A reply naming a written file is the path, a headline count, and one next action.
- A reply at a plan pause says why each choice was made and what wasn't verified. The hook's review sheet already lists the files and checks.
- A commit body or PR description is the shortest structured form that's complete.
- Still over the ceiling after cutting, cut again. Never explain the length.

<!-- chat: end -->

- Keep, one sentence each, a tradeoff that flips the decision, a risk that bites later, a warning before an irreversible action, and what wasn't run or was ruled out.
- Never write a preamble, recap, closing summary, offer to help, unasked alternatives or next steps, a tradeoff section, or a "what I didn't do" section.
- Numbered steps, a state line, and a time estimate appear only in a multi-step task the reader is executing.

## 1. Structure

- Never more than 4 prose lines in a row without a blank line, a list, or a header.
- Three or more comparable items go in a table.
- The answer, decisions, and questions go together in the final message after the last tool call.
- A code you assigned mid-session gets replaced by the name of the thing.

<!-- chat: skip -->

- Long reasoning goes to a file, never the terminal.

<!-- chat: end -->

## 2. Mechanics

- A code block carries a language tag.
- A command offered as the next action was run first. Not run, describe it in words instead.

<!-- chat: skip -->

## 3. Where output goes

- A file you author goes through Write or Edit at an absolute path. Never a shell redirect, `sed -i`, or `sd`.
- A temporary file goes to the directory `kit scratch-dir` prints (`skills/kit-rules/reference/tooling.md` section 3).
- A deliverable (a PR description, commit draft, email, spec, doc, or report) is a file, and its path is the reply's first line.
- `/write commit`, `/write devnote`, and `/write explainer` print to the terminal, per their own Output sections.
- After writing a report, plan, or review, the reply is its path, headline counts, and one next action. Never restate its findings.

<!-- chat: end -->

## 4. External communication

Applies to commit messages, PR descriptions, devnotes, review comments, stakeholder writeups, and release notes, with no detailed-explanation exception.

- Write for a senior reader who knows the project better than you.
- Report what you checked and what it showed. Offer a suggestion as an option, never an instruction.
- Point to the right place (the file, the function, the follow-up) instead of explaining it. A sentence that neither informs nor points goes.
- Use a plain chat register. No preamble, compliments, recap, sign-off, headers, bold labels, or triads.
- Name the change and its reason. Replace a generic reason ("improve readability", "address feedback") with what changed and why.
- Keep a line naming what wasn't run. Cut a line naming what was preserved.
- A review comment carries one concern in one to three sentences. It leads with the evidence (a count, a repro and its rate, a `file:line`, the input that breaks it) and ends with the fix as a question. A non-blocking point says so first, a nit starts with "Nit," and names the replacement, and what's right gets conceded in the same comment.
