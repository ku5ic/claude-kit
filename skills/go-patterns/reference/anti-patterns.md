# Anti-patterns

Severity levels: the report-format skill.

- An error discarded with `_`: `failure`. Check it, or document why ignoring it is safe.
- `err == ErrX` or a type assertion on a possibly wrapped error: `failure`. Use `errors.Is`, `errors.As`, or `errors.AsType`.
- A goroutine with no exit condition: `failure`. Tie it to `ctx.Done()`, a closed channel, or a `WaitGroup` the caller waits on.
- Wrapping with `%v` where callers inspect the cause: `warning`. Use `%w`.
- A Context stored in a struct field: `warning`. Pass `ctx` as the first parameter.
- An interface declared next to its only implementation: `info`. Return the concrete type; callers declare the interface they need.
