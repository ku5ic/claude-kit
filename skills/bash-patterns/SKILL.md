---
name: bash-patterns
description: Bash and shell patterns for safety flags, quoting, traps, and ShellCheck. Use for .sh, .bash, or .zsh files, shebang scripts, bin/ scripts, dotfiles, Makefile recipes, or shell questions.
---

# Bash patterns

- Default assumption: bash 5.x targeting macOS (via Homebrew bash) and Linux.
- Scripts use `#!/usr/bin/env bash`, so the resolved binary is whatever is first on PATH -- on the user's machine that's the Homebrew 5.x build, not the stock macOS `/bin/bash` 3.2.
- Anything that requires bash 4.0+ (case modification, `mapfile`) or 4.4+ (`${parameter@op}`) is unsafe under stock macOS bash; flagged here when the version cut matters.

## Reference files

| File                                                                 | Covers                                                                                                       |
| -------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| [reference/safety-flags.md](reference/safety-flags.md)               | Script preamble, `set -euo pipefail` edge cases, `inherit_errexit`, `nullglob`, verification before shipping |
| [reference/quoting.md](reference/quoting.md)                         | Quoting rules, nested quoting, heredoc indentation, `printf` over `echo`, zsh-vs-bash differences            |
| [reference/conditionals.md](reference/conditionals.md)               | `[[ ]]` over `[ ]`, glob vs literal, numeric vs string comparison, `(( ))`, `case`                           |
| [reference/functions-and-traps.md](reference/functions-and-traps.md) | Functions and locals, `trap` patterns, `BASH_LINENO` / `BASH_SOURCE` / `caller` debug introspection          |
| [reference/parameter-expansion.md](reference/parameter-expansion.md) | `${var:-}`, `${var:?}`, prefix/suffix removal, pattern substitution, case modification                       |
| [reference/subshells-and-pipes.md](reference/subshells-and-pipes.md) | Subshell semantics, `\|` pitfalls, process substitution `<(cmd)`, filesystem operations                      |
| [reference/shellcheck.md](reference/shellcheck.md)                   | Top SC codes, suppression discipline, POSIX-only contexts (Alpine, BusyBox)                                  |
| [reference/anti-patterns.md](reference/anti-patterns.md)             | Fifteen review-time anti-patterns with severity calls                                                        |
