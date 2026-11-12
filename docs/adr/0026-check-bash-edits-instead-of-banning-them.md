# 26. Check Bash edits instead of banning them

Date: 2026-11-12

## Status

Accepted

## Context

Since 909e23e, `output.md` sent every authored file through Write or Edit, never a shell redirect, `sed -i`, or `sd`. Its reason in `docs/rules.md`: only Write and Edit fired the hooks that guard, sanitize, and format written files. The ban was prose, so nothing stopped a `sed -i`, and the file it changed skipped every check. 26014ed began checking Bash edits at Stop.

## Decision

`bash-edits`, a PostToolUse hook on Bash (`hooks/hooks.json`), gives every file a Bash command changed the checks an Edit gets: slop, weaken, imports, and tokens, then sanitize and format (`go/internal/hooks/bashedits.go`). A failure goes back to Claude. `output.md` drops the ban, and names a file by its absolute path whichever tool writes it. Made in 664d02b.

## Consequences

- A mechanical edit can go through `sed`, a heredoc, or a script and still gets the checks.
- The checks run after the write, so a failure is sent back, where an Edit's guard stops it before (`docs/rules.md`).
- A file an Edit or Write left as it is now is skipped. The baseline is the tree after the last Bash command checked, else the turn's start.
