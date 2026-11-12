# Context and concurrency

- Counting goroutines on 1.25+: `wg.Go(func() { ... })`, not `wg.Add(1)` plus `go func() { defer wg.Done() ... }()`.
- A `go` directive of 1.22+: each loop iteration gets new variables, so drop `v := v` copies. Below 1.22, keep them.
- A function that needs a Context: take it as the first parameter, named `ctx`. Never store it in a struct.
- Context values: only request-scoped data that crosses processes and APIs, never optional parameters.
- Starting a goroutine: give it an exit, such as `ctx.Done()`, a closed channel, or a `WaitGroup` the caller waits on.
