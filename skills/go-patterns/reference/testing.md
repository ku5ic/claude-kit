# Testing

- Test output files on 1.26+: write them to `T.ArtifactDir` (or `B`, `F`), never next to the test source.
- Concurrent code with timeouts on 1.25+: test it with `testing/synctest`, on virtual time, with no real sleeps.
- `go test` runs a subset of vet checks; 1.27 added `stdversion` to it.
- Code under test that takes a context: pass `t.Context()`, never `context.Background()`. It's canceled just before `Cleanup` runs, and each subtest gets a fresh one.
