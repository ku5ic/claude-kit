# Parameter expansion

All forms below are bash-standard; the case-modification family is bash 4.0+.

- `${var:-default}` returns `default` if `var` is unset or empty; does not assign.
- `${var:=default}` assigns `default` to `var` if unset or empty, then expands.
- `${var:?error}` writes `error` to stderr and exits non-interactive shells if `var` is unset or empty. Useful at the top of a script to assert required env vars.
- `${var:+alt}` returns `alt` if `var` is set and non-empty, otherwise nothing.
- `${var#prefix}` / `${var##prefix}`: remove shortest / longest matching prefix.
- `${var%suffix}` / `${var%%suffix}`: remove shortest / longest matching suffix.
- `${var/pattern/replacement}` first match; `${var//pattern/replacement}` all matches.
- `${var^^}` uppercase, `${var,,}` lowercase. Bash 4.0+; unsafe under stock macOS bash 3.2.

## References

- GNU Bash manual (Shell Parameter Expansion): https://www.gnu.org/software/bash/manual/bash.html#Shell-Parameter-Expansion
