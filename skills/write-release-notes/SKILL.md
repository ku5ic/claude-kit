---
name: write-release-notes
description: Release notes from a branch's commits since its base
argument-hint: "<optional: explicit range like main..HEAD, or base branch name>"
allowed-tools:
  - Bash(git status *)
  - Bash(git diff *)
  - Bash(git log *)
  - Bash(git branch --show-current)
  - Bash(git rev-parse *)
---

# /write-release-notes

Tone and shape: `skills/kit-rules/reference/output.md` section 4, which wins over this file; the sections this file defines stay.

<!-- chat: only
In claude.ai chat, the arguments are the user's message and the material is what they pasted. Nothing can be run: if the diff, log, or text this needs isn't in the conversation, ask for it.
-->

<!-- chat: skip -->

Generate release notes from commits unique to the current branch vs its base.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                                   | Read                                                               |
| ---------------------------------------- | ------------------------------------------------------------------ |
| Grouping, wording, or dating the entries | [reference/changelog-practice.md](reference/changelog-practice.md) |

## Context

Run each and read its output: the current branch (`git branch --show-current`), the detected base (`kit git-base`), and the commits on this branch but not on the base (`kit git-base --log --no-merges`).

## Procedure

1. Interpret $ARGUMENTS:
   - If it looks like an explicit range (`ref..HEAD`, `sha1..sha2`): trust it. Pull commits with `git log --oneline --no-merges $ARGUMENTS`.
   - If it is a single ref (e.g. `develop`): use it as the explicit base. Pull commits with `git log --oneline --no-merges "$ARGUMENTS"..HEAD`.
   - If empty or not a valid git ref: the Context block used the auto-detected base; use those commits.
2. The commit list in the Context block is authoritative unless $ARGUMENTS specified an explicit range.

<!-- chat: end -->

<!-- chat: only
In claude.ai chat, generate release notes from the commit log the user pasted. There's no repo to pull a range from, so the paste is the whole list.
-->

3. For conventional-commits-style history (subjects like `feat:`, `fix:`, `chore:`), group by type. Otherwise group by inferred category (user-facing, internal, tooling).
4. Filter out noise: formatting-only commits, CI-only changes, revert pairs that cancel out, chore commits with no user impact.
5. Rewrite each kept commit into a user-facing line. "fix: correct date parsing for non-ISO inputs" becomes "Dates in non-ISO formats now parse correctly."

## Output file

<!-- chat: skip -->

Write to the path `kit scratch-dir release-notes <branch-or-range-slug>` prints.

<!-- chat: end -->

Structure:

```markdown
# Release <branch or range>

Base: <detected base branch or explicit range>
Range: <base>..HEAD (N commits)

## Added

<Omit if empty.>

## Changed

<Omit if empty.>

## Fixed

<Omit if empty.>

## Deprecated

<Omit if empty.>

## Removed

<Omit if empty.>

## Security

<CVE references if any. Omit if empty.>

## Internal

<Infrastructure, tooling, refactors that do not affect users. Keep short or omit.>
```

## Rules

- User-facing language. The audience is users of the project, not the authors.

<!-- chat: skip -->

- Signatures: `skills/kit-rules/reference/workflow.md` section 1.

<!-- chat: end -->

- No commit SHAs in the output unless the project convention includes them.
- Skip merge commits, formatting commits, and dependency bumps unless a bump is itself the release reason.
- If the range has nothing worth noting, say so and propose skipping the release.
- If a commit is ambiguous ("fix stuff", "wip"): flag it in an "Unclear" section rather than inventing intent.

<!-- chat: skip -->

- If `kit git-base` resolved no base, note that in the report and fall back to last 30 commits by running `git log --oneline --no-merges -30` explicitly.

<!-- chat: end -->

- Each entry is a single short line, never a paragraph per change.
