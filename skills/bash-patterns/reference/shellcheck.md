# ShellCheck discipline

- Every committed script: run ShellCheck.
- A finding: fix it, never silence it. SC2086, the unquoted expansion, is almost always a real bug.
- A suppression you can't avoid: `# shellcheck disable=SCXXXX` on the line above, with a comment saying why. Never a blanket disable at the file top.
- A file without a clear shebang: `# shellcheck shell=bash`. POSIX-only checks: `--shell sh`.
- A script's target: bash, with `#!/usr/bin/env bash` and `set -euo pipefail`, using arrays, `[[ ]]`, `${var^^}`, and process substitution freely. The Google Shell Style Guide allows only bash for executables and keeps POSIX `sh` for an external constraint.
- POSIX `sh` only for a script that runs where bash doesn't: an Alpine or BusyBox image whose `/bin/sh` is ash (unless the image installs bash), or early init and recovery scripts. State the choice at the top of the file.
