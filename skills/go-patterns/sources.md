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

## reference/anti-patterns.md

### Claims

- An error discarded with `_`, `failure`. The failure goes unnoticed and the next line r...: https://go.dev/wiki/CodeReviewComments#handle-errors
- `err == ErrX` or a type assertion on a possibly wrapped error, `failure`. Once anything wraps the error with `%w`, the c...: https://pkg.go.dev/errors
- Wrapping with `%v` where callers inspect the cause, `warning`. `%v` turns the cause into text, so `errors.Is`...: https://pkg.go.dev/fmt#Errorf
- A Context stored in a struct field, `warning`. It ties cancellation to the object's lifetime...: https://pkg.go.dev/context
- A `CancelFunc` that isn't called on every path, `failure`. The child context leaks until its parent is ca...: https://pkg.go.dev/context
- A goroutine with no exit condition, `failure`. A goroutine blocked forever on a channel is a...: https://go.dev/wiki/CodeReviewComments#goroutine-lifetimes
- An interface declared next to its only implementation, `info`. Interfaces belong with the consumer. Return the c...: https://go.dev/wiki/CodeReviewComments#interfaces

## reference/context-and-concurrency.md

### Claims

- Context rules: https://pkg.go.dev/context
- Goroutine lifetimes, When you start a goroutine, make it clear when, or whethe...: https://go.dev/wiki/CodeReviewComments#goroutine-lifetimes
- `WaitGroup.Go` (1.25+), `wg.Go(func() { ... })` creates and counts the goroutine...: https://go.dev/doc/go1.25, https://go.dev/doc/go1.27
- Loop variables (1.22+), From Go 1.22, each loop iteration creates new variables....: https://go.dev/doc/go1.22

## reference/errors.md

### Claims

- Check every error, Don't discard an error with `_`. If a function returns an...: https://go.dev/wiki/CodeReviewComments#handle-errors
- Wrap with `%w`, inspect with `errors`: https://pkg.go.dev/errors, https://go.dev/doc/go1.26
- Error strings, Lowercase, unless the first word is a proper noun or acro...: https://go.dev/wiki/CodeReviewComments#error-strings
- Don't panic for normal errors, Use an `error` return and multiple return values.: https://go.dev/wiki/CodeReviewComments#dont-panic
- No in-band errors, Don't signal failure with a special value like `-1`, `""`...: https://go.dev/wiki/CodeReviewComments#in-band-errors
- Indent the error path, Handle the error first and return early, keeping the norm...: https://go.dev/wiki/CodeReviewComments#indent-error-flow

## reference/style.md

### Claims

- Code Review Comments: https://go.dev/wiki/CodeReviewComments
- Newer language and stdlib features, row 1.22: https://go.dev/doc/go1.22
- Newer language and stdlib features, row 1.22: https://go.dev/doc/go1.22
- Newer language and stdlib features, row 1.26: https://go.dev/doc/go1.26
- Newer language and stdlib features, row 1.27: https://go.dev/doc/go1.27
- Newer language and stdlib features, row 1.27: https://go.dev/doc/go1.27
- Newer language and stdlib features, row 1.27: https://go.dev/doc/go1.27
- Newer language and stdlib features, `go fix` (rebuilt in 1.26) applies modernizers built on t...: https://go.dev/doc/go1.26

## reference/testing.md

### Claims

- `t.Context()`, A test's context is canceled just before its `Cleanup` fu...: https://github.com/golang/go/blob/master/src/testing/testing.go
- `testing/synctest` (1.25+), Generally available since 1.25, after an experiment in 1....: https://go.dev/doc/go1.25
- Test artifacts (1.26+), `T.ArtifactDir`, `B.ArtifactDir`, and `F.ArtifactDir` ret...: https://go.dev/doc/go1.26
- Vet runs inside `go test`, `go test` runs a subset of vet checks automatically; 1.27...: https://go.dev/doc/go1.27
