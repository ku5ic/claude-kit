# 32. Run git and bash by name, and test where PATH is read

Date: 2026-11-14

## Status

Accepted

Amends [30. Kit tools and formatters resolve from PATH through resolve](0030-kit-tools-and-formatters-resolve-from-path-through-resolve.md)

## Context

[30](0030-kit-tools-and-formatters-resolve-from-path-through-resolve.md) names `kittool.Path` "the one place a PATH-only copy runs", and its last consequence says calling `exec.LookPath` elsewhere bypasses both resolvers. Some by-name runs sit outside that rule, and 30 doesn't name them: `git` (`proc.Command(Timeout, "git", args...)` in `go/internal/git/git.go`), `bash` (`go/internal/run/run.go` and `go/internal/hooks/format.go`), and `go` inside resolve itself (`go/internal/resolve/policy.go`). Nothing enforced the rule, and 30 exists because seven packages had drifted into by-name runs.

## Decision

Code outside `go/internal/resolve` runs only `git` and `bash` by name. Every other binary goes through `kittool.Path` or `Resolver.Bin`. `exec.LookPath` outside resolve stays allowed only in `searchPath` in `go/internal/run/run.go`, which checks whether PATH already finds a resolved copy and runs nothing it finds.

`TestBinariesRunByNameOnlyWhereADR0030Says` in `go/internal/resolve/byname_test.go` parses every non-test Go file under `go/internal` and `go/cmd`. It fails on any other `exec.LookPath`, and on a string-literal binary passed to `proc.Command`, `exec.Command`, or `exec.CommandContext`.

## Consequences

- A new by-name run fails the test. It needs a change to this decision, or a move behind `kittool.Path`.
- A binary named by a variable isn't caught. The test reads literals only.
