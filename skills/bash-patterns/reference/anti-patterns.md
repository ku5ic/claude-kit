# Anti-patterns to flag

- Parsing `ls`: `for f in $(ls)`. Breaks on filenames with spaces, newlines, or glob characters. `failure`. Use `for f in *` (with `shopt -s nullglob` to handle empty globs) or `find ... -print0 | xargs -0`.
- `for f in $(find ...)`: same word-splitting hazard. `failure`. Use `find -print0 | xargs -0` or `while IFS= read -r -d '' f; do ...; done < <(find ... -print0)`.
- Useless `cat`: `cat file | grep ...`. `warning`.
- `cmd | while read line; do var=...; done` then reading `var` outside the loop. Pipe creates a subshell; assignment is lost. `failure`. Use input redirection or process substitution.
- Unquoted expansions: `[ $var = "yes" ]`. `failure`. With unset or empty `$var` this becomes `[ = "yes" ]`, a syntax error. Quote it, or use `[[ ]]`.
- Missing `set -euo pipefail` in non-trivial scripts. `warning`. Failure modes are silent.
- Hardcoded `/tmp/<name>`: predictable, racy, and not cleaned up. `warning`. `mktemp -d` plus a trap.
- `rm -rf "$VAR/"` without verifying `$VAR` is non-empty. `failure`. Empty `$VAR` deletes `/`. Guard with `[[ -n "$VAR" ]]` or use `${VAR:?}` to error out on empty.
- Functions without `local`. `warning`. Caller scope leaks; remote-debug nightmare.
- `eval` on user input. `failure` unless every component is provably safe; even then, prefer arrays of arguments and avoid `eval` entirely.
- `cd "$dir"` without `|| exit` or subshell wrap. `warning`. SC2164.
- `<<EOF` when the heredoc body should be literal (paths, regex, code samples). `warning`. Use `<<'EOF'`.
- `read` without `-r`. `warning`. The default mangles backslashes and treats `\<newline>` as line continuation.
- `[[ "$x" == "yes" ]] && do_thing || do_other_thing`. `warning`. The `||` runs if `do_thing` itself fails, not just if the condition is false. Use `if/then/else`.
- Globs without `nullglob`: `for f in *.log` runs once with the literal string `*.log` if no files match. `warning`. `shopt -s nullglob` at the top, or guard inside: `[[ -e "$f" ]] || continue`.

## References

- BashPitfalls: https://mywiki.wooledge.org/BashPitfalls
