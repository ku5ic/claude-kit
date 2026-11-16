# 4. Release 0.3: resolve tools from the project, block only on changed lines

Date: 2026-10-07

## Status

Accepted

## Context

Release 0.3 (v0.3.0, PR #7) chose where the kit's binaries come from and which findings block. This record replaces the former ADRs 9 and 11, with what 0.4.0 (PR #8) tightened. Its sources are theirs.

### Tool resolution

The kit resolved tool binaries three different ways, so one tool could run at two versions depending on the feature (PR #7). A project that pinned prettier 3.6.2 got the global 3.9.6 in one place, with 253 against 281 failing files.

### Changed lines

Stop runs a project's checks on the files Claude edited. An edited file can carry findings on lines the turn never touched.

## Decision

### Tool resolution

One resolver, `go/internal/resolve`, picks every binary the kit runs. For a project's own command, the order is the project's own copy, a `go.mod` tool directive, the package manager's environment, then a pin from asdf, mise, or a Brewfile. It refuses to fetch (`npx`, `pnpm dlx`, `uv run`), and it refuses a copy that's only on PATH without a pin. Every check prints the binary it ran and where it came from.

### Changed lines

A lint-like check blocks only on changed lines. Dead-code tools (knip, vulture, deadcode, tflint's unused declarations) block only on lines changed since the git base, and their findings decide, not their exit codes, since these tools disagree on them.

## Consequences

### Tool resolution

- The kit's result matches the project's own CI and editor, since all run the same version.
- A declared but missing tool skips with the install command instead of running a different version.
- A new feature that runs a tool goes through the resolver. Tools the kit runs for itself, which no project pins, later got their own door inside resolve, `kittool.Path` (`go/internal/resolve/kittool`), and `go/internal/resolve/byname_test.go` fails on a binary run by name anywhere else but `git` and `bash`.

### Changed lines

- A finding on an untouched line doesn't block the turn.
- A check has to report line numbers the kit can match to the diff.
- Debt in the files Claude edits stays until someone fixes it on purpose.
