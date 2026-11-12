# bash-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- GNU Bash manual: https://www.gnu.org/software/bash/manual/bash.html
- POSIX shell command language: https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html
- ShellCheck wiki: https://www.shellcheck.net/wiki/
- BashFAQ: https://mywiki.wooledge.org/BashFAQ
- BashPitfalls: https://mywiki.wooledge.org/BashPitfalls
- BashGuide: https://mywiki.wooledge.org/BashGuide
- Google Shell Style Guide: https://google.github.io/styleguide/shellguide.html

### Version notes

Checked: 2026-11-10 against https://ftp.gnu.org/gnu/bash/, https://tiswww.case.edu/php/chet/bash/NEWS, the bash 5.3.20 manual page, https://github.com/koalaman/shellcheck/blob/master/CHANGELOG.md

- 5.3 (2025-07-30; Homebrew 5.3.20): no 6.0 release exists. Corrected per the manual: `inherit_errexit` covers command substitution only, and every command of a multi-command pipeline runs in a subshell. ShellCheck 0.11.0 (2025-08-04) current; it disables SC2002 (useless cat) by default.
- 4.4: `shopt -s inherit_errexit`, `${parameter@op}` (corrected from 4.0 per NEWS). 4.0: case modification, `mapfile`.

### Left to linters

Checked: 2026-11-11 with ShellCheck 0.11.0, run with no flags and no rc file (`--norc`) on a probe script. These rules were dropped because a default check reports them; don't add them back.

- SC2006 reports legacy backticks.
- SC2015 reports `A && B || C` used as if-then-else.
- SC2030 and SC2031 report a variable set in a pipeline's subshell and read after the loop.
- SC2044 reports a `for` loop over `find` output.
- SC2045 reports a `for` loop over `ls` output.
- SC2046 reports an unquoted command substitution.
- SC2086 reports an unquoted expansion, including `[ $var = "yes" ]`.
- SC2115 reports `rm -rf "$VAR/"`, which an empty `$VAR` turns into `/`.
- SC2155 reports `local foo=$(cmd)`, which masks the command's status.
- SC2162 reports `read` without `-r`.
- SC2164 reports `cd` without `|| exit`.

Not caught by defaults, so kept: useless `cat` (SC2002 is off), a missing `set -euo pipefail`, a hardcoded `/tmp` path, a function variable without `local`, `eval`, an unquoted heredoc delimiter, a glob without `nullglob`, `[ ]` over `[[ ]]` (`require-double-brackets` is optional), `==` on numbers, `echo -e`, `"${array[*]}"` outside a `for` loop, the `function` keyword.

## reference/anti-patterns.md

### Claims

- Review-time pitfalls: https://mywiki.wooledge.org/BashPitfalls
- SC2002 off by default in ShellCheck 0.11.0: https://github.com/koalaman/shellcheck/blob/master/CHANGELOG.md

## reference/conditionals.md

### Claims

- `[[ ]]`, `==` patterns, `(( ))`, `case`: https://www.gnu.org/software/bash/manual/bash.html#Conditional-Constructs
- Tests and conditionals: https://mywiki.wooledge.org/BashGuide/TestsAndConditionals

## reference/functions-and-traps.md

### Claims

- Functions and `local`: https://www.gnu.org/software/bash/manual/bash.html#Shell-Functions
- `BASH_LINENO`, `BASH_SOURCE`, `BASH_SUBSHELL`: https://www.gnu.org/software/bash/manual/bash.html#Bash-Variables

## reference/parameter-expansion.md

### Claims

- Every form, case modification 4.0+: https://www.gnu.org/software/bash/manual/bash.html#Shell-Parameter-Expansion

## reference/quoting.md

### Claims

- Quote kinds, ANSI-C quoting, heredoc delimiters: https://www.gnu.org/software/bash/manual/bash.html#Quoting
- `printf` over `echo`: https://mywiki.wooledge.org/BashFAQ/050
- zsh modifiers, the tied `path` array, and `NOMATCH`: https://zsh.sourceforge.io/Doc/Release/Expansion.html, https://zsh.sourceforge.io/Doc/Release/Parameters.html. Checked 2026-11-12 in zsh: `$head:src/x` gave "bad substitution", `typeset -p path` printed `typeset -aT PATH path`, and `echo foo?state=open` gave "no matches found".

## reference/safety-flags.md

### Claims

- `set -e`, `set -u`, `pipefail`, shopts: https://www.gnu.org/software/bash/manual/bash.html
- `set -e` exceptions: https://mywiki.wooledge.org/BashFAQ/105
- `lastpipe`: https://mywiki.wooledge.org/BashFAQ/024

## reference/shellcheck.md

### Claims

- Suppression and shell directives: https://www.shellcheck.net/wiki/
- SC2086: https://www.shellcheck.net/wiki/SC2086
- Bash only for executables: https://google.github.io/styleguide/shellguide.html

## reference/subshells-and-pipes.md

### Claims

- Process substitution: https://www.gnu.org/software/bash/manual/bash.html#Process-Substitution
