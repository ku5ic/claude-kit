# Anti-patterns to flag

- Useless `cat`, as in `cat file | grep ...`: `warning`. ShellCheck 0.11.0+ reports it (SC2002) only with `--enable=useless-use-of-cat`.
- `eval` on user input: `failure`, unless every component is provably safe. Even then, use an array of arguments and no `eval`.
- A non-trivial script without `set -euo pipefail`: `warning`.
- A hardcoded `/tmp/<name>`: `warning`. Use `mktemp -d` plus a trap.
- A function variable without `local`: `warning`.
- `<<EOF` on a body that must stay literal (paths, regex, code samples): `warning`. Use `<<'EOF'`.
- A glob such as `for f in *.log` without `nullglob`: `warning`. With no match, the loop runs once on the literal `*.log`. Set `shopt -s nullglob` at the top, or guard with `[[ -e "$f" ]] || continue`.
