# Parameter expansion

- `${var^^}` uppercases, `${var,,}` lowercases. Bash 4.0+, so unsafe under stock macOS bash 3.2. Every other form here is standard bash.
- `${var:-default}`: `default` if `var` is unset or empty. Doesn't assign.
- `${var:=default}`: assigns `default` to `var` if it's unset or empty, then expands.
- `${var:?error}`: writes `error` to stderr and exits a non-interactive shell if `var` is unset or empty. Assert required env vars with it at the top of a script.
- `${var:+alt}`: `alt` if `var` is set and non-empty, otherwise nothing.
- `${var#prefix}` and `${var##prefix}` remove the shortest and longest matching prefix; `${var%suffix}` and `${var%%suffix}` the suffix.
- `${var/pattern/replacement}` replaces the first match, `${var//pattern/replacement}` every match.
