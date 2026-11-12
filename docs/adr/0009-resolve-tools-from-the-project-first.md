# 9. Resolve tools from the project first

Date: 2026-10-07

## Status

Accepted

## Context

The kit resolved tool binaries three different ways, so one tool could run at two versions depending on the feature (PR #7). A project that pinned prettier 3.6.2 got the global 3.9.6 in one place, with 253 against 281 failing files.

## Decision

One resolver, `go/internal/resolve`, picks every binary the kit runs. The order is the project's own copy, a `go.mod` tool directive, the package manager's environment, then a pin from asdf, mise, or a Brewfile. It refuses to fetch (`npx`, `pnpm dlx`, `uv run`), and it refuses a copy that's only on PATH without a pin. Every check prints the binary it ran and where it came from. Made in 0.3.0 (PR #7) and tightened in 0.4.0 (PR #8).

## Consequences

- The kit's result matches the project's own CI and editor, since all run the same version.
- A declared but missing tool skips with the install command instead of running a different version.
- A new feature that runs a tool goes through the resolver. Calling a binary by name bypasses all of this.
