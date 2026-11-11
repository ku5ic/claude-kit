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

## reference/anti-patterns.md

### References

- BashPitfalls: https://mywiki.wooledge.org/BashPitfalls

## reference/conditionals.md

### References

- GNU Bash manual (Conditional Constructs): https://www.gnu.org/software/bash/manual/bash.html#Conditional-Constructs
- BashGuide on tests: https://mywiki.wooledge.org/BashGuide/TestsAndConditionals

## reference/functions-and-traps.md

### References

- GNU Bash manual (Functions): https://www.gnu.org/software/bash/manual/bash.html#Shell-Functions
- GNU Bash manual (Bash Variables, BASH_LINENO etc.): https://www.gnu.org/software/bash/manual/bash.html#Bash-Variables
- ShellCheck SC2164 (`cd ... || exit`): https://www.shellcheck.net/wiki/SC2164

## reference/parameter-expansion.md

### References

- GNU Bash manual (Shell Parameter Expansion): https://www.gnu.org/software/bash/manual/bash.html#Shell-Parameter-Expansion

## reference/quoting.md

### References

- GNU Bash manual (Quoting): https://www.gnu.org/software/bash/manual/bash.html#Quoting
- BashFAQ #50 (echo vs printf): https://mywiki.wooledge.org/BashFAQ/050
- ShellCheck SC2086: https://www.shellcheck.net/wiki/SC2086

## reference/safety-flags.md

### References

- GNU Bash manual (`set` builtin): https://www.gnu.org/software/bash/manual/bash.html
- BashFAQ on `set -e`: https://mywiki.wooledge.org/BashFAQ/105
- ShellCheck SC2155 (declare-and-assign-separately): https://www.shellcheck.net/wiki/SC2155

## reference/shellcheck.md

### References

- ShellCheck wiki: https://www.shellcheck.net/wiki/
- ShellCheck SC2086: https://www.shellcheck.net/wiki/SC2086
- ShellCheck SC2155: https://www.shellcheck.net/wiki/SC2155
- ShellCheck SC2046: https://www.shellcheck.net/wiki/SC2046
- ShellCheck SC2164: https://www.shellcheck.net/wiki/SC2164
- Google Shell Style Guide: https://google.github.io/styleguide/shellguide.html

## reference/subshells-and-pipes.md

### References

- GNU Bash manual (Process Substitution): https://www.gnu.org/software/bash/manual/bash.html#Process-Substitution
- GNU Bash manual (Command Substitution): https://www.gnu.org/software/bash/manual/bash.html#Command-Substitution
- BashFAQ #24 (variables in pipelines): https://mywiki.wooledge.org/BashFAQ/024
- ShellCheck SC2164 (cd-without-exit): https://www.shellcheck.net/wiki/SC2164
