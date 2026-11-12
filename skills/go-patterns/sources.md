# go-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- Go Code Review Comments: https://go.dev/wiki/CodeReviewComments
- Effective Go: https://go.dev/doc/effective_go
- Release notes: https://go.dev/doc/devel/release
- Package context: https://pkg.go.dev/context

### Version notes

Checked: 2026-11-10 against https://go.dev/dl/?mode=json and the release notes at https://go.dev/doc/go1.27, https://go.dev/doc/go1.26, https://go.dev/doc/go1.25

- 1.27 (1.27.4 current; 1.26.11 the other supported release): generic methods; `encoding/json` backed by the v2 implementation, with `GOEXPERIMENT=nojsonv2` as a temporary opt-out; new `uuid` package.
- 1.26: `new` accepts an expression; `errors.AsType`; `go fix` rebuilt around modernizers; `T.ArtifactDir`.
- 1.25: `sync.WaitGroup.Go`; `testing/synctest` generally available; vet's `waitgroup` and `hostport` analyzers.

### Left to linters

Checked: 2026-11-11 with golangci-lint 2.14.0 defaults (errcheck, govet, ineffassign, staticcheck, unused) on a probe module. These rules were dropped because a default check reports them; don't add them back.

- govet `lostcancel` reports a `CancelFunc` not called on every path.
- staticcheck `SA1012` reports a nil Context.
- staticcheck `ST1005` reports a capitalized or punctuated error string.
- staticcheck `ST1006` reports `this` or `self` as a receiver name.
- vet `waitgroup` (1.25, renamed `waitgroupgo` in 1.27) reports a misplaced `WaitGroup.Add`.

Not caught by defaults, so kept: `_` discards (errcheck skips blank assignments), `err ==` on wrapped errors, `%v` wraps, ctx in a struct, initialism case (`ST1003` is off), receiver consistency (`ST1016` is off), `else` after return, `v := v` copies.

## reference/errors.md

### Claims

- `errors.AsType` 1.26+, `%w`, `errors.Is`, `errors.Join`: https://pkg.go.dev/errors, https://go.dev/doc/go1.26
- Check every error: https://go.dev/wiki/CodeReviewComments#handle-errors
- No panic for normal errors: https://go.dev/wiki/CodeReviewComments#dont-panic
- No in-band errors: https://go.dev/wiki/CodeReviewComments#in-band-errors
- Error path first, no `else` after return: https://go.dev/wiki/CodeReviewComments#indent-error-flow

## reference/context-and-concurrency.md

### Claims

- `WaitGroup.Go` 1.25+: https://go.dev/doc/go1.25
- Per-iteration loop variables 1.22+: https://go.dev/doc/go1.22
- Context as first parameter, not in a struct; context values only for request-scoped data: https://pkg.go.dev/context
- Goroutine exits: https://go.dev/wiki/CodeReviewComments#goroutine-lifetimes

## reference/style.md

### Claims

- 1.27 rows (generic methods, json v2 and `nojsonv2`, `uuid`): https://go.dev/doc/go1.27
- 1.26 rows (`new(expr)`, `go fix` modernizers): https://go.dev/doc/go1.26
- 1.22 rows (range over int, ServeMux patterns): https://go.dev/doc/go1.22
- Interfaces, receiver names, initialisms, package names, named results: https://go.dev/wiki/CodeReviewComments

## reference/testing.md

### Claims

- `T.ArtifactDir` 1.26+: https://go.dev/doc/go1.26
- `testing/synctest` GA in 1.25: https://go.dev/doc/go1.25
- `stdversion` in `go test`'s vet set, 1.27: https://go.dev/doc/go1.27
- `t.Context()` canceled before Cleanup, fresh per subtest: https://github.com/golang/go/blob/master/src/testing/testing.go

## reference/anti-patterns.md

### Claims

- `_` discard, goroutine lifetimes, interface placement: https://go.dev/wiki/CodeReviewComments
- `err ==` and type assertions on wrapped errors: https://pkg.go.dev/errors
- `%v` vs `%w`: https://pkg.go.dev/fmt#Errorf
- Context in a struct: https://pkg.go.dev/context
