# Functions, locals, traps, and cleanup

## Contents

- [Functions and locals](#functions-and-locals)
- [Traps and cleanup](#traps-and-cleanup)
- [Debug introspection](#debug-introspection)

## Functions and locals

- `name() { ... }` is the POSIX form; `function name { ... }` is bash-specific. Use `name() { ... }` unless there is a reason not to.
- Every variable inside a function gets `local`. Without it, the assignment writes to the caller's scope or to global, which is a major footgun.
- `readonly` for constants set at script load.
- Functions return integer status codes (0 success, non-zero failure). `return N` exits the function; `exit N` exits the script.
- Stdout for data, stderr for human messages: `echo "result"`, `echo "error: ..." >&2`. Tests pipe stdout; logs pipe stderr.
- `local foo=$(cmd)` masks `cmd`'s exit status (the `local` builtin's success is what survives). Split into `local foo; foo=$(cmd)` when the exit status matters. ShellCheck flags this as SC2155.

## Traps and cleanup

- `trap 'cleanup' EXIT` runs `cleanup` whenever the shell exits, error or not. Use this for tempdir removal.
- `trap 'echo interrupted >&2; exit 130' INT TERM` for graceful shutdown. 130 is `128 + 2` (SIGINT signal number), the conventional exit code.
- `trap '...' ERR` runs only on non-zero exit, with the same edge-case list as `set -e`. Less reliable than `EXIT` for cleanup.
- Combined: `trap 'rc=$?; cleanup; exit $rc' EXIT` preserves the original exit code through the cleanup.
- Cleanup functions: idempotent and defensive. `rm -f`, not `rm`. `kill -0 "$pid" 2>/dev/null && kill "$pid"`, not bare `kill`.

## Debug introspection

When a script fails in CI or in production and the error message alone is not enough, bash exposes a handful of variables and builtins that pinpoint the failure site.

- `$BASH_LINENO` is an array of line numbers in the call chain. `${BASH_LINENO[0]}` is the line in the parent that called the current function. Useful inside an `ERR` trap to log where a failure originated rather than where the trap fired.
- `$BASH_SOURCE` is the parallel array of source filenames. `${BASH_SOURCE[0]}` is the file containing the current function.
- `$BASH_SUBSHELL` is the subshell nesting level. Equal to 0 in the main shell; increments inside `(...)`, command substitution, and pipeline non-final stages. Distinguishing "I am running in the parent shell" from "I am running in a subshell" matters for variable-survival reasoning.
- `caller [N]` builtin prints the line, function, and source file at depth N in the call chain. Pair with `BASH_LINENO` for human-readable backtraces in error reporters.

A useful pattern for production scripts:

```sh
err_report() {
    local exit_code=$?
    echo "error: line ${BASH_LINENO[0]} in ${BASH_SOURCE[1]}: exit $exit_code" >&2
}
trap err_report ERR
```

This logs the exact line where `set -e` triggered, not the line where the trap was installed.

## References

- GNU Bash manual (Functions): https://www.gnu.org/software/bash/manual/bash.html#Shell-Functions
- GNU Bash manual (Bash Variables, BASH_LINENO etc.): https://www.gnu.org/software/bash/manual/bash.html#Bash-Variables
- ShellCheck SC2164 (`cd ... || exit`): https://www.shellcheck.net/wiki/SC2164
