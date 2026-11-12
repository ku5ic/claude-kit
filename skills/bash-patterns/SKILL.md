---
name: bash-patterns
description: Bash and shell patterns for safety flags, quoting, traps, and ShellCheck. Load before any edit to a .sh, .bash, or .zsh file, shebang script, or Makefile recipe, however small, or for shell questions.
---

# Bash patterns

Default assumption: bash 5.x on macOS (Homebrew bash) and Linux.

- Bash 4.4+ (`${parameter@op}`) and 4.0+ (case modification, `mapfile`) are unsafe under stock macOS `/bin/bash` 3.2.
- `#!/usr/bin/env bash` runs the first bash on PATH. On the user's machine that's the Homebrew 5.x build, not stock 3.2.

## Reference files

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                                                                        | Read                                                                 |
| --------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| Starting a script, or setting `set` flags, shopts, or an `inherit_errexit` preamble           | [reference/safety-flags.md](reference/safety-flags.md)               |
| Quoting a variable, building an array, writing a heredoc, or printing with `printf` or `echo` | [reference/quoting.md](reference/quoting.md)                         |
| Writing an `if`, `[[ ]]`, `(( ))`, or `case` test                                             | [reference/conditionals.md](reference/conditionals.md)               |
| Writing a function, a `trap`, or cleanup code                                                 | [reference/functions-and-traps.md](reference/functions-and-traps.md) |
| Using `${var:-default}`, `${var:?}`, case modification, or prefix and suffix removal          | [reference/parameter-expansion.md](reference/parameter-expansion.md) |
| Using a pipe, a subshell, `<(cmd)`, `realpath`, or a filesystem operation                     | [reference/subshells-and-pipes.md](reference/subshells-and-pipes.md) |
| Silencing a ShellCheck finding, or writing for POSIX sh (Alpine, BusyBox)                     | [reference/shellcheck.md](reference/shellcheck.md)                   |
| Writing, changing, or reviewing bash code                                                     | [reference/anti-patterns.md](reference/anti-patterns.md)             |
