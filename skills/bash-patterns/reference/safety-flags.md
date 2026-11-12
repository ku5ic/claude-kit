# Safety flags and verification

- A script relying on `set -e` inside `$(...)`, on bash 4.4+: `shopt -s inherit_errexit` after `set -euo pipefail`. Outside posix mode, bash otherwise clears `errexit` in command substitution.
- A non-trivial script starts with `#!/usr/bin/env bash`, `set -euo pipefail`, and `IFS=$'\n\t'`.
- The shebang: `#!/usr/bin/env bash`, which picks up Homebrew bash on macOS. `#!/bin/bash` only for a script that runs before PATH is set up, such as early init or a recovery shell.
- `set -e` exits on a simple command's non-zero status, except in an `if`/`elif` test or a `while`/`until` head, in an `&&`/`||` chain before its final operator, in any pipeline command but the last (subject to `pipefail`), and after a leading `!`. An `ERR` trap fires under the same conditions.
- A function called in a conditional, as in `if my_func; then`, runs with `set -e` suspended. To enforce `errexit` there, check failures explicitly in its body.
- `set -e` alone isn't enough: pair `set -euo pipefail` with explicit checks at decision boundaries.
- `set -u` errors on an unset variable. `$@`, `$*`, and `[@]`/`[*]` subscripts are exempt, so `"$@"` with no arguments is safe.
- `set -o pipefail` makes a pipeline's status the rightmost non-zero instead of the last command's.
- `IFS=$'\n\t'` drops space from the default space-tab-newline, so a filename with spaces doesn't split.
- A glob that may match nothing: `shopt -s nullglob` expands it to nothing, `shopt -s failglob` makes it an error. Pick `failglob` when no match is a hard failure.
- Assignments that must survive a pipeline's last command: `shopt -s lastpipe`, with job control off (`set +m`).
- Non-obvious shell (case-pattern globs, regex, IFS games, complex parameter expansion, heredoc quoting): run it with `bash -c '...'` before committing. ShellCheck is static; a run with real inputs is the final check.
