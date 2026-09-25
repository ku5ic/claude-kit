---
name: bash-patterns
description: Bash and shell script patterns - safety flags, quoting, conditionals, traps, functions, ShellCheck discipline, and review-worthy anti-patterns. Use whenever the project contains `.sh`/`.bash`/`.zsh` files, files with a bash/sh shebang, scripts in `bin/`, shell dotfiles, or Makefile recipes, OR the user asks about shell scripting or a shell pipeline, even if "bash" is not mentioned by name.
---

# Bash patterns

- Default assumption: bash 5.x targeting macOS (via Homebrew bash) and Linux.
- Scripts use `#!/usr/bin/env bash`, so the resolved binary is whatever is first on PATH -- on the user's machine that's the Homebrew 5.x build, not the stock macOS `/bin/bash` 3.2.
- Anything that requires bash 4.0+ (case modification, `mapfile`, `${parameter@op}`) is unsafe under stock macOS bash; flagged here when the version cut matters.

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

## References

- GNU Bash manual: https://www.gnu.org/software/bash/manual/bash.html
- POSIX shell command language: https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html
- ShellCheck wiki: https://www.shellcheck.net/wiki/
- BashFAQ: https://mywiki.wooledge.org/BashFAQ
- BashPitfalls: https://mywiki.wooledge.org/BashPitfalls
- BashGuide: https://mywiki.wooledge.org/BashGuide
- Google Shell Style Guide: https://google.github.io/styleguide/shellguide.html

## Version notes

Checked: 2026-09-12 against https://ftp.gnu.org/gnu/bash/ and https://formulae.brew.sh/formula/bash

- 5.3 (2025-07-30; Homebrew 5.3.15): no guidance change; no 6.0 release exists.
- 4.4: `shopt -s inherit_errexit`. 4.0: case modification, `mapfile`, `${parameter@op}`. Both cuts unverified on 2026-09-12 (NEWS file unreachable); left as written.
