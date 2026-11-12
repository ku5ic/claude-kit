---
name: go-patterns
description: Go patterns for errors, context, goroutines, interfaces, recent release features, and tests. Load before any edit to a .go file or go.mod, however small, and for Go questions.
---

# Go patterns

Default assumption: Go 1.27.

- A feature applies when the `go` directive in `go.mod` is at least its version, whatever the installed toolchain. Check the directive before suggesting one; loop variables (1.22) are the sharpest case.
- Code Review Comments and the `context` docs hold on every version.

## Reference files

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                                                                   | Read                                                                         |
| ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| Writing or changing error handling, wrapping, or a panic                                 | [reference/errors.md](reference/errors.md)                                   |
| Starting a goroutine, or using a context, a `WaitGroup`, or a loop variable in a closure | [reference/context-and-concurrency.md](reference/context-and-concurrency.md) |
| Naming something, placing an interface, or using a feature from a recent release         | [reference/style.md](reference/style.md)                                     |
| Writing or changing a test                                                               | [reference/testing.md](reference/testing.md)                                 |
| Writing, changing, or reviewing Go code                                                  | [reference/anti-patterns.md](reference/anti-patterns.md)                     |
