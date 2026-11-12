# Subshells, pipes, process substitution, filesystem operations

- `realpath path` and `readlink -f path` exist on macOS 13+ and Linux. Their flags differ between BSD and GNU, so write the no-flag `realpath path`.
- Process substitution `<(cmd)` and `>(cmd)` gives a command's output or input a filename. Bash, ksh, and zsh only, not POSIX.
- A consumer that takes a path, such as `diff`, `comm`, or `paste`: `diff <(grep -v '^#' file_a) <(grep -v '^#' file_b)`, no temp files. A consumer that reads stdin gets a plain pipe.
- Bash backs `<(cmd)` with `/dev/fd/N`, or a named pipe where `/dev/fd` is missing. Nothing lands on disk, and there's no window between create and read.
- Useless `cat`: `grep ... file` or `< file grep ...`, not `cat file | grep ...`.
- A temp directory: `mktemp -d`, which works on BSD (macOS) and GNU, with a `trap` cleanup.
- A `cd` the rest of the script must not see: `(cd "$dir" && cmd)`.
- Filenames with spaces or newlines: `find ... -print0 | xargs -0 cmd`. Both flags exist on BSD and GNU.
