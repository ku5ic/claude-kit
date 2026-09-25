# Subshells, pipes, process substitution, filesystem operations

## Contents

- [Subshells, pipes, process substitution](#subshells-pipes-process-substitution)
- [Process substitution: when and why](#process-substitution-when-and-why)
- [Filesystem operations](#filesystem-operations)

## Subshells, pipes, process substitution

- `$(cmd)` is the modern command substitution form. Backticks still work; they are harder to nest and harder to read. Stay with `$(...)`.
- The right side of `|` runs in a subshell. Variable assignments inside `cmd | while read -r line; do count=$((count+1)); done` do not survive the loop. Replace with `while read -r line; do ...; done < file` (input redirection) or `while read -r line; do ...; done < <(cmd)` (process substitution, same effect with arbitrary commands).
- Process substitution `<(cmd)` produces a path the consumer reads from; bash, ksh, zsh only, not POSIX.
- Useless `cat`: `cat file | grep ...` spawns an extra process. Use `grep ... file` or `< file grep ...`.

## Process substitution: when and why

Process substitution `<(cmd)` and `>(cmd)` give a command output (or input) the shape of a filename so that consumers expecting a path -- like `diff`, `comm`, `paste` -- can work on streams without temp files. Two practical reasons to reach for it:

- Avoiding temp files: `diff <(grep -v '^#' file_a) <(grep -v '^#' file_b)` is one line; the temp-file equivalent is three lines plus cleanup.
- Avoiding the pipe-subshell trap: `while read -r line; do ...; done < <(cmd)` keeps the loop body in the parent shell, so variable assignments survive after the loop ends. Compare to `cmd | while read -r line; do count=$((count+1)); done` where the loop body's `count` is lost.

Implementation detail worth knowing: bash uses `/dev/fd/N` (a numbered file descriptor) on systems that support it, falling back to a named-pipe (FIFO) if `/dev/fd` is not available. The consumer sees a filename either way; the substitution is byte-stream-equivalent to a temp file but with no on-disk artifact and no race window between create-and-read.

When NOT to reach for process substitution: when the consumer can take stdin directly (`cmd | grep something` is fine; `grep something <(cmd)` is the same with extra ceremony). Use it when the consumer expects a path argument.

## Filesystem operations

- `mktemp -d` for temp directories; `-d` is portable on BSD (macOS) and GNU. Pair with `trap` cleanup.
- `cd "$dir" || exit 1` to fail fast. Bare `cd` continues silently in the wrong directory if the path is missing or unreadable. ShellCheck flags this as SC2164.
- `(cd "$dir" && cmd)` keeps the `cd` scoped to a subshell when the surrounding script must stay in its original cwd.
- Filenames with spaces or newlines: `find ... -print0 | xargs -0 cmd`. Both `-print0` and `-0` exist on BSD (macOS) and GNU. The `for f in $(find ...)` pattern is broken on any non-trivial input.
- `realpath path` and `readlink -f path` are both available on modern macOS (13+) and Linux. Flag sets differ between BSD and GNU implementations; the no-flag `realpath path` form is the safest portable spelling.

## References

- GNU Bash manual (Process Substitution): https://www.gnu.org/software/bash/manual/bash.html#Process-Substitution
- GNU Bash manual (Command Substitution): https://www.gnu.org/software/bash/manual/bash.html#Command-Substitution
- BashFAQ #24 (variables in pipelines): https://mywiki.wooledge.org/BashFAQ/024
- ShellCheck SC2164 (cd-without-exit): https://www.shellcheck.net/wiki/SC2164
