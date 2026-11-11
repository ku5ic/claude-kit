# Functions, locals, traps, and cleanup

## Functions and locals

- Defining a function: `name() { ... }`, the POSIX form, unless there's a reason for bash's `function name { ... }`.
- Every variable inside a function: `local`. Without it, the assignment writes to the caller's scope or the global one.
- A constant set at script load: `readonly`.
- A function returns an integer status, 0 for success. `return N` exits the function; `exit N` exits the script.
- Data goes to stdout, human messages to stderr: `echo "error: ..." >&2`.

## Traps and cleanup

- Cleanup such as removing a tempdir: `trap 'cleanup' EXIT`, which runs on every exit. Not `ERR`, which fires only on a non-zero status and carries `set -e`'s exceptions.
- Keeping the original exit code through cleanup: `trap 'rc=$?; cleanup; exit $rc' EXIT`.
- Graceful shutdown: `trap 'echo interrupted >&2; exit 130' INT TERM`. 130 is 128 plus SIGINT's number, 2.
- A cleanup function: idempotent and defensive. `rm -f`, not `rm`; `kill -0 "$pid" 2>/dev/null && kill "$pid"`, not bare `kill`.

## Debug introspection

- `${BASH_LINENO[0]}` is the line in the caller that called the current function. In an `ERR` trap handler it logs where the failure happened, not where the trap fired.
- `${BASH_SOURCE[0]}` is the file holding the current function; `BASH_SOURCE` runs parallel to `BASH_LINENO`.
- `$BASH_SUBSHELL` is 0 in the main shell. It increments inside `(...)`, command substitution, and every command of a multi-command pipeline; the last runs in the shell only with `lastpipe` on and job control off.
- `caller [N]` prints the line, function, and source file at depth N. Pair it with `BASH_LINENO` for readable backtraces.

An `ERR` handler that logs the line where `set -e` fired:

```sh
err_report() {
    local exit_code=$?
    echo "error: line ${BASH_LINENO[0]} in ${BASH_SOURCE[1]}: exit $exit_code" >&2
}
trap err_report ERR
```
