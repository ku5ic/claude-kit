---
name: go-patterns
description: Go patterns for errors, context, goroutines, interfaces, recent release features, and tests. Use for .go files, go.mod, or Go questions.
---

# Go patterns

Default assumption: Go 1.27.

- A feature applies when the `go` directive in `go.mod` is at least its version, whatever the installed toolchain. Check the directive before suggesting one; loop variables (1.22) are the sharpest case.
- Code Review Comments and the `context` docs hold on every version.

## Reference files

| File                                                                         | Covers                                                                     |
| ---------------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| [reference/errors.md](reference/errors.md)                                   | `errors.AsType`, `%w` wrapping, `errors.Is`/`Join`, panics, in-band errors |
| [reference/context-and-concurrency.md](reference/context-and-concurrency.md) | `WaitGroup.Go`, loop variables, context rules, goroutine lifetimes         |
| [reference/style.md](reference/style.md)                                     | Features by version, then naming and interface placement                   |
| [reference/testing.md](reference/testing.md)                                 | Artifacts, `synctest`, vet in `go test`, `t.Context`                       |
| [reference/anti-patterns.md](reference/anti-patterns.md)                     | Six review-time anti-patterns with severity calls                          |
