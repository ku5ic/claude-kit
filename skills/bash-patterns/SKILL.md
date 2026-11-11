---
name: bash-patterns
description: Bash and shell patterns for safety flags, quoting, traps, and ShellCheck. Use for .sh, .bash, or .zsh files, shebang scripts, bin/ scripts, dotfiles, Makefile recipes, or shell questions.
---

# Bash patterns

Default assumption: bash 5.x on macOS (Homebrew bash) and Linux.

- Bash 4.4+ (`${parameter@op}`) and 4.0+ (case modification, `mapfile`) are unsafe under stock macOS `/bin/bash` 3.2.
- `#!/usr/bin/env bash` runs the first bash on PATH. On the user's machine that's the Homebrew 5.x build, not stock 3.2.

## Reference files

| File                                                                 | Covers                                                                                |
| -------------------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| [reference/safety-flags.md](reference/safety-flags.md)               | `inherit_errexit`, the preamble, `set -euo pipefail` edge cases, shopts, verification |
| [reference/quoting.md](reference/quoting.md)                         | Quote kinds, arrays, nesting, heredocs, `printf` over `echo`, zsh vs bash             |
| [reference/conditionals.md](reference/conditionals.md)               | `[[ ]]` over `[ ]`, glob vs literal, numeric vs string, `(( ))`, `case`               |
| [reference/functions-and-traps.md](reference/functions-and-traps.md) | Functions and locals, `trap` patterns, `BASH_LINENO` / `BASH_SOURCE` / `caller`       |
| [reference/parameter-expansion.md](reference/parameter-expansion.md) | Case modification, defaults, `${var:?}`, prefix/suffix removal, substitution          |
| [reference/subshells-and-pipes.md](reference/subshells-and-pipes.md) | `realpath`, process substitution `<(cmd)`, filesystem operations                      |
| [reference/shellcheck.md](reference/shellcheck.md)                   | Suppression discipline, bash vs POSIX-only contexts (Alpine, BusyBox)                 |
| [reference/anti-patterns.md](reference/anti-patterns.md)             | Seven review-time anti-patterns with severity calls                                   |
