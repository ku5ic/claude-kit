# ShellCheck discipline

ShellCheck catches most quoting and word-splitting bugs statically. Run it on every committed script. Codes worth knowing:

- `SC2086` -- "Double quote to prevent globbing and word splitting." The unquoted-expansion finding. Almost always a real bug; fix it, do not silence it.
- `SC2155` -- "Declare and assign separately to avoid masking return values." `local`/`export`/`readonly` with a command substitution on the same line drops the command's exit status. Split the declaration.
- `SC2046` -- "Quote this to prevent word splitting." Unquoted command substitution on the right of an assignment or in argument position.
- `SC2164` -- "Use `cd ... || exit` in case `cd` fails." Bare `cd` without an exit-on-failure guard.

Suppression: `# shellcheck disable=SCXXXX` on the line above the offending line, with a comment explaining why. Do not blanket-disable at file top; that hides real findings. The default answer to a ShellCheck warning is to fix it.

## Portability stance and POSIX-only contexts

ShellCheck supports a `# shellcheck shell=bash` directive for files without a clear shebang and a `--shell sh` flag for POSIX-only checks. Choose deliberately.

Most scripts in a developer's environment can target bash and rely on bash features (arrays, `[[ ]]`, `${var^^}`, process substitution). The Google Shell Style Guide takes the same position: bash is the only permitted scripting language for executables, and POSIX `sh` is reserved for cases where an external constraint forces it.

Two contexts where POSIX-only is the right call:

- **Alpine / BusyBox shells.** Container base images that ship `/bin/sh` linked to BusyBox's ash (or a similar minimal shell) do not have bash. A script intended to run in those containers either needs a bash install in the image or needs to stay POSIX. Pick one and document it at the top of the file.
- **Early init or recovery scripts.** Anything that runs before the package manager has installed bash, or in a recovery shell with a stripped-down environment, must use POSIX features only.

For everything else: bash with `#!/usr/bin/env bash` and `set -euo pipefail`. The portability cost is zero on a developer machine and a CI runner; the safety wins are large.

## References

- ShellCheck wiki: https://www.shellcheck.net/wiki/
- ShellCheck SC2086: https://www.shellcheck.net/wiki/SC2086
- ShellCheck SC2155: https://www.shellcheck.net/wiki/SC2155
- ShellCheck SC2046: https://www.shellcheck.net/wiki/SC2046
- ShellCheck SC2164: https://www.shellcheck.net/wiki/SC2164
- Google Shell Style Guide: https://google.github.io/styleguide/shellguide.html
