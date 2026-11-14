# 30. Kit tools and formatters resolve from PATH through resolve

Date: 2026-11-14

## Status

Accepted

Amends [9. Resolve tools from the project first](0009-resolve-tools-from-the-project-first.md)

## Context

[9](0009-resolve-tools-from-the-project-first.md) says `go/internal/resolve` picks every binary the kit runs and refuses a copy that's only on PATH without a pin. Seven packages called a binary by name instead:

- the on_edit formatters (`onPath` in `go/internal/hooks/format.go`), which `kit.yml` documents as "the first command on PATH formats it"
- ast-grep (`astGrep` in `go/internal/codegraph/imports.go`, from 8cc6a1b)
- gitleaks (`go/internal/bashguard/commit.go`)
- just (`go/internal/sources/extract.go`)
- the registry lookups' npm and go (`go/internal/pkgcheck/lookup.go`)
- claude (`go/internal/claims/claims.go`, `go/internal/gapfill/ask.go`)

No project pins these, so `Resolver.Bin` refuses them, and on some systems `sg` is shadow-utils' newgrp alias, not ast-grep.

## Decision

`resolve.KitTool(names...)` is the one place a PATH-only copy runs: the first of names found on PATH, and an `sg` only when its `--version` names ast-grep. It serves the tools the kit runs for itself (ast-grep, gitleaks, claude, the registry lookups) and the on_edit formatters. A command a project defines (a task, a check, just) goes through `Resolver.Bin`. A package below resolve's layer takes the resolved path from its caller.

## Consequences

- Where PATH is allowed is answered in one package again.
- A kit tool runs whatever version PATH has, as before; only a project's own commands get 9's pin check.
- A new tool the kit runs for itself goes through `KitTool`, a project's command through `Bin`. Calling `exec.LookPath` elsewhere bypasses both.
