---
name: go-patterns
description: Go patterns - error wrapping and inspection, context and goroutine lifetimes, Code Review Comments naming and interface placement, features from recent releases (generic methods, new(expr), errors.AsType, WaitGroup.Go, ServeMux patterns, json v2), testing helpers, and review-time anti-patterns. Use whenever the project contains `.go` files or a `go.mod`, OR the user asks about Go, goroutines, channels, contexts, or Go error handling, even if Go is not mentioned by name.
---

# Go patterns

Default assumption: Go 1.27.

- Language features apply according to the `go` directive in `go.mod`, not the installed toolchain. Loop-variable semantics (1.22) are the sharpest case. Check the directive before suggesting a feature.
- Rules from Code Review Comments and the `context` package docs hold for every version.

## Reference files

| File                                                                         | Covers                                                                                  |
| ---------------------------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| [reference/errors.md](reference/errors.md)                                   | `%w` wrapping, `errors.Is`/`As`/`AsType`/`Join`, error strings, panics, in-band errors  |
| [reference/context-and-concurrency.md](reference/context-and-concurrency.md) | Context rules, cancel funcs, goroutine lifetimes, `WaitGroup.Go`, loop variables         |
| [reference/style.md](reference/style.md)                                     | Naming, interface placement, and newer language and stdlib features by version          |
| [reference/testing.md](reference/testing.md)                                 | `t.Context`, `synctest`, artifacts, vet checks `go test` runs                           |
| [reference/anti-patterns.md](reference/anti-patterns.md)                     | Seven review-time anti-patterns with severity calls                                     |

## References

- Go Code Review Comments: https://go.dev/wiki/CodeReviewComments
- Effective Go: https://go.dev/doc/effective_go
- Release notes: https://go.dev/doc/devel/release
- Package context: https://pkg.go.dev/context

## Version notes

Checked: 2026-09-26 against https://go.dev/dl/?mode=json and the release notes at https://go.dev/doc/go1.27, https://go.dev/doc/go1.26, https://go.dev/doc/go1.25

- 1.27 (1.27.1 current): generic methods; `encoding/json` backed by the v2 implementation, with `GOEXPERIMENT=nojsonv2` as a temporary opt-out; new `uuid` package.
- 1.26: `new` accepts an expression; `errors.AsType`; `go fix` rebuilt around modernizers; `T.ArtifactDir`.
- 1.25: `sync.WaitGroup.Go`; `testing/synctest` generally available; vet's `waitgroup` and `hostport` analyzers.
