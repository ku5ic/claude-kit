# Safety flags and verification

## Script preamble

Every non-trivial script starts with:

```sh
#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'
```

- `set -e` exits on a simple-command non-zero status. It does not exit when the failure is:
  - In the test of an `if`/`elif`, or the head of `while`/`until`.
  - In `&&`/`||` chains except after the final operator.
  - In any pipeline command but the last (subject to `pipefail`).
  - Preceded by a leading `!`.
  - A trap on `ERR` runs under the same conditions.
- `set -u` errors on parameter expansion of an unset variable. `$@` and `$*` (and array `[@]`/`[*]` subscripts) are exempt by design, so iterating `"$@"` over zero positional args is safe.
- `set -o pipefail` makes a pipeline's exit status the rightmost non-zero, instead of just the last command's status. Without it, `cmd_a | cmd_b` succeeds whenever `cmd_b` succeeds, even if `cmd_a` failed.
- `IFS=$'\n\t'` removes space as a field separator.
  - Default IFS is space-tab-newline; dropping space prevents accidental word splitting on filenames-with-spaces.
  - `$'...'` is ANSI-C quoting, supported in bash since 2.x.

`#!/usr/bin/env bash` over `#!/bin/bash`. The env form picks up Homebrew bash on macOS so 4.0+ features work; the absolute path is locked to whatever ships in `/bin`. Use the absolute form only when the script must run before PATH is set up (early init scripts, recovery shells).

## set -e edge cases worth memorizing

The list of conditions under which `set -e` does NOT exit is the most common source of confusion. The five practical traps:

1. **Inside an `if` test or `while`/`until` head.** `if cmd; then ...` does not exit even if `cmd` fails; `cmd`'s status is the test result. Same applies to `&&`/`||` chains except after the final operator.
2. **Non-final pipeline commands without `pipefail`.** `failing_cmd | grep something` succeeds based on `grep`. With `pipefail`, the pipeline's exit code is the rightmost non-zero.
3. **Command substitution masking.** `local foo=$(maybe_fail)` succeeds because `local`'s success is what `set -e` sees. ShellCheck SC2155. Split: `local foo; foo=$(maybe_fail)`.
4. **Subshells need `inherit_errexit`.** A `(set -e is not inherited by subshells unless explicitly set)` subshell may have its own `set -e` state. Bash 4.4 added `shopt -s inherit_errexit` so subshells (including command substitution) see the parent's `errexit`.
5. **Functions invoked in conditional context.** A function called as `if my_func; then ...` runs with `set -e` effectively suspended for that call; failures inside the function do not exit. To enforce the parent's `errexit`, the function body must check explicitly.

These caveats are why `set -e` alone is not a complete safety story. `set -euo pipefail` plus explicit error checks at decision boundaries is what production scripts actually need.

## inherit_errexit and other useful shopts

Bash 4.4 introduced `shopt -s inherit_errexit`, which propagates `errexit` into command substitutions. Enable it after `set -euo pipefail` if the script relies on `set -e` to catch failures inside `$(...)` blocks:

```sh
set -euo pipefail
shopt -s inherit_errexit
```

Other shopts worth knowing:

- `shopt -s nullglob`: globs that match nothing expand to nothing (instead of the literal pattern). Without this, `for f in *.log` runs once with the literal `*.log` if no `.log` files exist.
- `shopt -s failglob`: globs that match nothing produce an error. Stricter alternative to `nullglob`; choose one based on whether "no matches" is a recoverable state or a hard fail.
- `shopt -s lastpipe`: the last command in a pipeline runs in the parent shell instead of a subshell, so variable assignments survive. Requires job control disabled (`set +m`).

## Verification before shipping

Non-obvious shell -- case-pattern globs, regex, IFS games, complex parameter expansion, here-doc quoting -- gets a quick test in `bash` before commit. A 30-second `bash -c '...'` run beats multiple debug rounds. ShellCheck is a static analyzer; runtime behavior with real inputs is the final check.

## References

- GNU Bash manual (`set` builtin): https://www.gnu.org/software/bash/manual/bash.html
- BashFAQ on `set -e`: https://mywiki.wooledge.org/BashFAQ/105
- ShellCheck SC2155 (declare-and-assign-separately): https://www.shellcheck.net/wiki/SC2155
