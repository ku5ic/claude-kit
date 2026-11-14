# 30. Kit tools and formatters resolve from PATH through resolve

Date: 2026-11-14

## Status

Accepted

Amends [9. Resolve tools from the project first](0009-resolve-tools-from-the-project-first.md)

Amended by [32. Run git and bash by name, and test where PATH is read](0032-run-git-and-bash-by-name-and-test-where-path-is-read.md)

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

`kittool.Path(names...)`, in `go/internal/resolve/kittool`, is the one place a PATH-only copy runs: the first of names found on PATH, and an `sg` only when its `--version` names ast-grep. It serves the tools the kit runs for itself (ast-grep, gitleaks, claude, the registry lookups, `just --summary` listing recipes) and the on_edit formatters. Running a project's command goes through `Resolver.Bin`. kittool sits at layer 1, below sources and pkgcheck, which resolve itself imports, so every caller reaches it directly.

## Consequences

- Where PATH is allowed is answered under one folder again.
- A kit tool runs whatever version PATH has, as before; only a project's own commands get 9's pin check.
- A new tool the kit runs for itself goes through `kittool.Path`, a project's command through `Bin`. Calling `exec.LookPath` elsewhere bypasses both.
