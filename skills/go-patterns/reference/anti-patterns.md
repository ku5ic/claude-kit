# Anti-patterns

Severity rubric:

- `failure`: a concrete defect or violation that should not ship.
- `warning`: a smell or pattern that compounds with other findings.
- `info`: a hardening opportunity or note, not a defect.

## An error discarded with `_`

`failure`. The failure goes unnoticed and the next line runs on a zero value. Check it, or document why it's safe to ignore. Source: https://go.dev/wiki/CodeReviewComments#handle-errors

## `err == ErrX` or a type assertion on a possibly wrapped error

`failure`. Once anything wraps the error with `%w`, the comparison stops matching. Use `errors.Is`, or `errors.As` / `errors.AsType`. Source: https://pkg.go.dev/errors

## Wrapping with `%v` where callers inspect the cause

`warning`. `%v` turns the cause into text, so `errors.Is` can't find it. Use `%w`. Source: https://pkg.go.dev/fmt#Errorf

## A Context stored in a struct field

`warning`. It ties cancellation to the object's lifetime instead of the call. Pass `ctx` as the first parameter. Source: https://pkg.go.dev/context

## A `CancelFunc` that isn't called on every path

`failure`. The child context leaks until its parent is canceled; `go vet` reports it. `defer cancel()` right after creating it. Source: https://pkg.go.dev/context

## A goroutine with no exit condition

`failure`. A goroutine blocked forever on a channel is a leak. Tie it to a `ctx.Done()`, a closed channel, or a `WaitGroup` the caller waits on. Source: https://go.dev/wiki/CodeReviewComments#goroutine-lifetimes

## An interface declared next to its only implementation

`info`. Interfaces belong with the consumer. Return the concrete type and let callers declare the interface they need. Source: https://go.dev/wiki/CodeReviewComments#interfaces
