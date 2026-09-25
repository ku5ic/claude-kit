# Quoting

The single highest-impact section. Most shell bugs are quoting bugs.

## Core rules

- Quote every variable expansion: `"$var"`, never bare `$var`. Unquoted expansions undergo word splitting on IFS and pathname expansion (globbing).
- Quote command substitutions: `cmd "$(other_cmd)"`. Same reason.
- Single quotes preserve every character literal; double quotes preserve everything except `$`, backticks, `\`, and `!`.
- Heredocs: `<<'EOF'` (any quoting on the delimiter) disables expansion; `<<EOF` allows parameter, command, and arithmetic expansion. Use `<<-EOF` to strip leading tabs (tabs only, not spaces).
- ANSI-C quoting `$'\n'`, `$'\t'`, `$'\x1b'` for escape characters in literals.
- Array expansions: `"${array[@]}"` expands each element to a separate word; `"${array[*]}"` joins with the first character of `IFS`. `[@]` is almost always what you want.

## Nested quoting

Nested quoting in command substitution is a recurring source of confusion. The rule: each `"..."` in `"$(cmd "$arg")"` belongs to its own lexical scope. The inner `"$arg"` is independently quoted within the `$(...)` substitution; the outer quotes apply to the result.

```sh
# safe: $arg may contain spaces; result is one word
result=$(grep "$arg" file.txt)

# safe: nested but each level is independently quoted
result=$(printf '%s\n' "$(date '+%Y-%m-%d')")
```

Two practical tips:

- ShellCheck handles nesting correctly; if it stays quiet, the quoting is correct.
- When nested quoting becomes hard to read, extract to a function or a temporary variable. Readability beats density.

## Heredoc indentation

`<<-EOF` strips leading tab characters from each line (and from the closing delimiter). Spaces are not stripped, only tabs. The intent is to allow heredocs inside indented function bodies without dictating column-zero alignment.

```sh
function generate_report() {
    cat <<-EOF
	header line one
	header line two
	EOF
}
```

The lines between `<<-EOF` and the trailing `EOF` are tab-indented; the runtime strips the tabs. If your editor inserts spaces instead, the strip silently fails and the body retains its leading whitespace. A common trap.

For literal content (paths, regex, code samples, anything that should NOT undergo expansion), quote the delimiter: `<<'EOF'`. The body is then verbatim.

## printf over echo

`echo`'s handling of `-e`, `-n`, and embedded backslashes varies across shells and even across bash versions. `printf '%s\n' "$str"` is portable and predictable. Reach for `echo` only for trivial constant strings where none of those edge cases apply.

```sh
# avoid: -e behavior depends on the shell and bash version
echo -e "a\tb"

# prefer: deterministic
printf 'a\tb\n'
printf '%s\n' "$str"
```

## zsh and bash compatibility notes

The user's interactive shell on macOS is zsh; scripts run under bash via the shebang. A handful of differences matter when a snippet is tested at the prompt and then dropped into a script:

- Word splitting: zsh does NOT split unquoted parameter expansions by default; bash does. A snippet that "works" interactively in zsh may fail at script time in bash if it relied on un-split behavior.
- Glob qualifiers: zsh accepts `*(N)` for nullglob-on-this-glob and `*(.)` for "regular files only"; bash does not. Use `shopt -s nullglob` and `[[ -f "$f" ]]` instead.
- Array indexing: zsh arrays are 1-indexed by default; bash arrays are 0-indexed. Set `setopt KSH_ARRAYS` in zsh for bash-compatible indexing, or test in bash directly.

When in doubt, run the script with `bash -c` (or `bash script.sh`) before committing.

## References

- GNU Bash manual (Quoting): https://www.gnu.org/software/bash/manual/bash.html#Quoting
- BashFAQ #50 (echo vs printf): https://mywiki.wooledge.org/BashFAQ/050
- ShellCheck SC2086: https://www.shellcheck.net/wiki/SC2086
