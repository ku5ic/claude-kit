# Testing

## `t.Context()`

A test's context is canceled just before its `Cleanup` functions run, so cleanup can wait on `ctx.Done()`. Each test and subtest gets a fresh context, not one derived from its parent. Pass `t.Context()` to code under test instead of `context.Background()`. Source: https://github.com/golang/go/blob/master/src/testing/testing.go

## `testing/synctest` (1.25+)

Generally available since 1.25, after an experiment in 1.24. It tests concurrent code with virtualized time, so a timeout test doesn't have to sleep for real. Source: https://go.dev/doc/go1.25

## Test artifacts (1.26+)

`T.ArtifactDir`, `B.ArtifactDir`, and `F.ArtifactDir` return the directory for test output files. Use it instead of writing next to the test source. Source: https://go.dev/doc/go1.26

## Vet runs inside `go test`

`go test` runs a subset of vet checks automatically; 1.27 added `stdversion` to that default set. The kit's `run-checks.sh` also runs `go vet ./...` and `go test ./...` wherever a `go.mod` is. Source: https://go.dev/doc/go1.27
