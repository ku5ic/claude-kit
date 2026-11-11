---
name: go-patterns
description: Go patterns for errors, context, goroutines, interfaces, recent release features, and tests. Use for .go files, go.mod, or Go questions.
---

# Go patterns

Default assumption: Go 1.27.

- A feature applies when the `go` directive in `go.mod` is at least its version, whatever the installed toolchain. Check the directive before suggesting one; loop variables (1.22) are the sharpest case.
- Code Review Comments and the `context` docs hold on every version.

## Reference files

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have.

| Before                                                                                   | Read                                                                         |
| ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| Writing or changing error handling, wrapping, or a panic                                 | [reference/errors.md](reference/errors.md)                                   |
| Starting a goroutine, or using a context, a `WaitGroup`, or a loop variable in a closure | [reference/context-and-concurrency.md](reference/context-and-concurrency.md) |
| Naming something, placing an interface, or using a feature from a recent release         | [reference/style.md](reference/style.md)                                     |
| Writing or changing a test                                                               | [reference/testing.md](reference/testing.md)                                 |
| Reviewing Go code                                                                        | [reference/anti-patterns.md](reference/anti-patterns.md)                     |
