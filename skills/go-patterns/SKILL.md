---
name: go-patterns
description: Go patterns for errors, context, goroutines, interfaces, recent release features, and tests. Use for .go files, go.mod, or Go questions.
---

# Go patterns

Default assumption: Go 1.27.

- Language features apply according to the `go` directive in `go.mod`, not the installed toolchain. Loop-variable semantics (1.22) are the sharpest case. Check the directive before suggesting a feature.
- Rules from Code Review Comments and the `context` package docs hold for every version.

## Reference files

| File                                                                         | Covers                                                                                 |
| ---------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| [reference/errors.md](reference/errors.md)                                   | `%w` wrapping, `errors.Is`/`As`/`AsType`/`Join`, error strings, panics, in-band errors |
| [reference/context-and-concurrency.md](reference/context-and-concurrency.md) | Context rules, cancel funcs, goroutine lifetimes, `WaitGroup.Go`, loop variables       |
| [reference/style.md](reference/style.md)                                     | Naming, interface placement, and newer language and stdlib features by version         |
| [reference/testing.md](reference/testing.md)                                 | `t.Context`, `synctest`, artifacts, vet checks `go test` runs                          |
| [reference/anti-patterns.md](reference/anti-patterns.md)                     | Seven review-time anti-patterns with severity calls                                    |
