# Context and concurrency

## Context rules

From the `context` package docs:

- Don't store a Context in a struct. Pass it explicitly to each function that needs it, as the first parameter, typically named `ctx`.
- Never pass a nil Context, even where a function permits it.
- Use context values only for request-scoped data that crosses processes and APIs, not for optional function parameters.
- Call the `CancelFunc` from `WithCancel`, `WithTimeout`, or `WithDeadline` on every path, usually with `defer cancel()`. Not calling it leaks the child context until the parent is canceled, and `go vet` checks for it.

Source: https://pkg.go.dev/context

## Goroutine lifetimes

When you start a goroutine, make it clear when, or whether, it exits. A goroutine blocked forever on a channel send or receive leaks. Source: https://go.dev/wiki/CodeReviewComments#goroutine-lifetimes

## `WaitGroup.Go` (1.25+)

`wg.Go(func() { ... })` creates and counts the goroutine in one call, replacing the `wg.Add(1)` / `go func() { defer wg.Done() ... }()` pattern. Vet's `waitgroup` analyzer (1.25) reports a misplaced `WaitGroup.Add`. In 1.27 it was renamed `waitgroupgo`. Sources: https://go.dev/doc/go1.25, https://go.dev/doc/go1.27

## Loop variables (1.22+)

From Go 1.22, each loop iteration creates new variables. This is enabled by the `go` directive in `go.mod` being 1.22 or later, and it ends the classic bug of goroutines or closures sharing one loop variable. Below 1.22 the `v := v` copy is still needed; at 1.22 and above it's noise. Source: https://go.dev/doc/go1.22
