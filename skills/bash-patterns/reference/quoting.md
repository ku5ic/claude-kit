# Quoting

- Single quotes keep every character literal. Double quotes keep all but `$`, backticks, `\`, and `!`.
- An array: `"${array[@]}"`, one word per element. `"${array[*]}"` joins the elements on the first character of `IFS`.
- Nested quotes: each `"..."` in `"$(cmd "$arg")"` is its own scope. The inner pair quotes the argument, the outer pair the result.
- Nested quoting ShellCheck doesn't flag: it's correct. Hard to read: extract a function or a temporary variable.
- A heredoc body that must stay literal (paths, regex, code samples): quote the delimiter, `<<'EOF'`. Unquoted `<<EOF` expands parameters, commands, and arithmetic.
- `<<-EOF` strips leading tabs, never spaces, from each body line and the closing delimiter. An editor that inserts spaces breaks the strip silently.
- Escape characters in a literal: ANSI-C quoting (bash 2.x+), `$'\n'`, `$'\t'`, `$'\x1b'`.
- Output with `-e`, `-n`, or backslashes: `printf`, as in `printf 'a\tb\n'` or `printf '%s\n' "$str"`. `echo` only for trivial constant strings.

## zsh and bash

The interactive shell on macOS is zsh; scripts run under bash. A snippet tested at the zsh prompt and dropped into a script:

- Word splitting: zsh doesn't split an unquoted expansion, bash does.
- Glob qualifiers: zsh's `*(N)` and `*(.)` don't exist in bash. Use `shopt -s nullglob` and `[[ -f "$f" ]]`.
- Arrays: zsh indexes from 1, bash from 0. `setopt KSH_ARRAYS` gives zsh bash's indexing.
- Before committing: run it with `bash script.sh` or `bash -c`.
