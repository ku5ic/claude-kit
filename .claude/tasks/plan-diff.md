# Plan diff: `kit run-checks --plan` against `--engine=enforce --plan`

Step 16b's acceptance. Each project's old and new plan lines (RUN and SKIP lines, commands under them), then every difference tagged **decision** (a recorded choice, named) or **regression** (to fix). None is left as a regression.

Fixtures are `go/internal/gapfill/testdata/*`, replayed from their recorded verdicts (prompt version 6); `KIT_PLAN_DIFF=<dir> go test ./internal/enforce -run TestPlanDiff` regenerates them. Both engines resolve against this machine (macOS, asdf, Homebrew), and no fixture has its packages installed.

Decisions referenced below:

- **D1** PATH-only binaries are refused unless pinned (`.tool-versions`, `mise.toml`, Brewfile) under their manager, a manifest's toolchain, a verified package manager, or the system's own (plan, Resolve).
- **D2** No toolchain or slot catalog: a kind comes from CI, then the task graph, the task runners, pre-commit, then evidence, per subproject; dead code runs only where something states it.
- **D3** One check per kind per subproject: a later tier fills only kinds the earlier ones left.
- **D4** An affected-only form runs when one verifies (plan, Full gate).
- **D5** An entry whose binary can't resolve SKIPs with the reason; nothing falls back to per-package tasks.
- **D6** Hook-manager entries (lint-staged, lefthook, husky, commitlint) are Stop and commit sources, not full-gate ones.
- **D7** A subproject with a manifest gets one line naming the kinds nothing enforces.

## claude-kit (this repo)

| Old                                                              | New                                                                                     |
| ---------------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| RUN ci: lint (shellcheck), `/opt/homebrew/bin/shellcheck` (PATH) | SKIP lint (ci.yml: shell/shellcheck): shellcheck only on PATH                           |
| RUN go: lint (ci.yml: golangci-lint) [go]                        | RUN lint (ci.yml: go/golangci-lint) [go], `--new-from-rev={base}`                       |
| RUN go: test (ci.yml: go) [go]                                   | RUN test (ci.yml: go/test) [go]                                                         |
| RUN go: vet [go]                                                 | RUN lint (ci.yml: go/vet) [go]                                                          |
| SKIP go: test [go] (covered)                                     | (no line)                                                                               |
| RUN go: deadcode [go], toolchain row                             | RUN deadcode (ci.yml: go/deadcode) [go], the CI step's own `$(...)` body                |
| (CI's semver and `claude plugin validate` steps unread)          | SKIP check (semver): jq only on PATH; SKIP check (plugin validate): claude only on PATH |
| (none)                                                           | RUN format-check (evidence go/go.mod) [go]: `test -z "$(gofmt -l .)"`                   |
| (none)                                                           | SKIP typecheck [go] (nothing enforces it)                                               |

- shellcheck, jq, claude SKIP: **decision D1**. The repo pins none of them; pinning (or a Brewfile) restores them.
- golangci-lint's affected-only form: **decision D4**.
- go vet from CI instead of the toolchain row, and no "covered" line: **decision D2**.
- Dead code from the CI step, `$(...)` and all: **decision** (plan, Gained: CI steps with pipes or `$(...)`).
- The semver and plugin-validate steps are now read: **decision** (Gained).
- format-check from evidence, typecheck line: **decisions D2, D7**.

## dotfiles

| Old                                                  | New                                                                                      |
| ---------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| RUN dotfiles: lint (lint.yml: shellcheck), PATH copy | RUN lint (lint.yml: shell/shellcheck), `find` (system) and shellcheck (pinned: Brewfile) |
| (bats unread)                                        | RUN test (lint.yml: shell/bats), bats (pinned: Brewfile `bats-core`)                     |

- shellcheck through `find -exec`: kept, now pinned by the Brewfile: **decision D1**.
- bats now runs: **decision** (Gained: a CI step's command no catalog named).

## go-noci (go.mod, no CI)

| Old                               | New                                                           |
| --------------------------------- | ------------------------------------------------------------- |
| RUN go: vet                       | RUN lint (evidence go.mod): `go vet ./...`                    |
| RUN go: test                      | RUN test (evidence go.mod): `go test ./...`                   |
| SKIP go: deadcode (not installed) | (no line)                                                     |
| (none)                            | RUN format-check (evidence go.mod): `test -z "$(gofmt -l .)"` |
| (none)                            | SKIP typecheck (nothing enforces it)                          |

- vet and test from evidence: **decision D2**.
- No dead-code line: nothing declares a dead-code tool: **decision D2**.
- format-check gained, typecheck line: **decisions D2, D7**.

## makefile (Makefile aggregate, go.mod)

| Old                                          | New                                                           |
| -------------------------------------------- | ------------------------------------------------------------- |
| RUN make: lint (`make lint`)                 | RUN lint (Makefile: lint): `make lint`                        |
| RUN make: test (`make test`)                 | RUN test (Makefile: test): `make test`                        |
| RUN go: vet                                  | RUN typecheck (evidence go.mod): `go vet ./...`               |
| SKIP go: test (covered by make: test)        | (no line)                                                     |
| SKIP make: typecheck, format-check, deadcode | RUN format-check (evidence go.mod): `test -z "$(gofmt -l .)"` |

- `make lint`, `make test` kept; `check` (an aggregate) isn't run on its own since its parts are: **decision D3**.
- go vet now fills typecheck, the kind the verdict gives it: **decision D2**.
- format-check from evidence: **decision D2**.

## lint-staged (package.json scripts and lint-staged, husky)

| Old                                                                            | New                                                      |
| ------------------------------------------------------------------------------ | -------------------------------------------------------- |
| RUN js: test (`npm run test`)                                                  | RUN test (package.json: test): `npm run test`            |
| SKIP js: typecheck, lint, format-check, deadcode (no task); knip not installed | SKIP lint, typecheck, format-check (nothing enforces it) |

- lint-staged's `prettier --check` doesn't fill format-check: **decision D6**.
- Evidence lint and format proposals are refused as `npm exec` with no local copy (nothing installed): **decision D1** (never fetch).
- One line for missing kinds, no knip line: **decisions D7, D2**.

## nx

| Old                           | New                                                                                                 |
| ----------------------------- | --------------------------------------------------------------------------------------------------- |
| SKIP js: every kind (no task) | SKIP lint (nx.json: lint), SKIP test (nx.json: test): nx declared in package.json but not installed |
| (none)                        | SKIP typecheck, format-check (nothing enforces it)                                                  |

- nx's targets are read, in their affected form, and SKIP until nx is installed: **decisions D4, D5** (plan, turbo/nx affected runs; the old engine needed `node_modules/.bin/nx` to see them at all).

## poetry

| Old                               | New                                                                                      |
| --------------------------------- | ---------------------------------------------------------------------------------------- |
| SKIP python: every kind (no task) | SKIP lint, typecheck, format-check, test (evidence pyproject.toml): poetry not installed |

- Evidence proposals through `poetry run`, skipped here because poetry isn't installed on this machine: **decisions D2, D1**.

## pre-commit

| Old                                                           | New                                                                       |
| ------------------------------------------------------------- | ------------------------------------------------------------------------- |
| RUN pre-commit: typecheck (`pre-commit run mypy --all-files`) | SKIP typecheck (.pre-commit-config.yaml: mypy): pre-commit not installed  |
| SKIP python: lint (no task)                                   | SKIP lint (.pre-commit-config.yaml: check-yaml): pre-commit not installed |
| SKIP python: format-check (no task)                           | SKIP format-check (evidence pyproject.toml): ruff not installed           |
| SKIP python: test, deadcode                                   | SKIP test (nothing enforces it)                                           |

- The old engine planned `pre-commit run` without pre-commit installed, to fail at run time; the new one SKIPs with the reason: **decision D1**.
- check-yaml fills lint: **decision D2** (its verdict's kind).

## rust

| Old                               | New                                                           |
| --------------------------------- | ------------------------------------------------------------- |
| RUN rust: check                   | RUN typecheck (evidence Cargo.toml): `cargo check`            |
| RUN rust: clippy `-- -D warnings` | RUN lint (evidence Cargo.toml): `cargo clippy -- -D warnings` |
| RUN rust: fmt `--check`           | RUN format-check (evidence Cargo.toml): `cargo fmt --check`   |
| RUN rust: test                    | RUN test (evidence Cargo.toml): `cargo test`                  |

- Same four commands, from evidence instead of the toolchain catalog: **decision D2**. Recording under prompt version 5 proposed `cargo clippy` without `-D warnings`, which can't fail; version 6 says a check must fail on a finding.

## turbo (pnpm workspace, turbo.json, packages/a)

| Old                                                   | New                                                                                     |
| ----------------------------------------------------- | --------------------------------------------------------------------------------------- |
| RUN js: lint, test (`pnpm run lint`, `pnpm run test`) | SKIP lint, test (turbo.json): turbo declared in package.json but not installed          |
| RUN js: lint, test [packages/a]                       | (covered by turbo, across every package)                                                |
| SKIP js: typecheck, format-check, deadcode            | SKIP typecheck, format-check (nothing enforces it)                                      |
| SKIP js: typecheck [packages/a]                       | RUN typecheck (evidence packages/a/package.json) [packages/a]: `pnpm exec tsc --noEmit` |

- turbo's own tasks in their affected form, which SKIP here until turbo is installed, and no fallback to the packages' scripts: **decisions D4, D5**.
- packages/a typecheck from evidence: **decision D2**. tsc isn't installed either; `pnpm exec` resolves only pnpm, and the executor (16c) reads exit 127 as SKIP not installed.

## yarn-pnp

| Old                             | New                                                                |
| ------------------------------- | ------------------------------------------------------------------ |
| RUN js: lint (`yarn run lint`)  | RUN lint (package.json: lint): `yarn run lint`                     |
| RUN js: test (`yarn run test`)  | RUN test (package.json: test): `yarn run test`                     |
| SKIP js: format-check (no task) | RUN format-check (evidence .prettierrc): `yarn prettier --check .` |
| SKIP js: typecheck, deadcode    | SKIP typecheck (nothing enforces it)                               |

- yarn from the verified manager facts instead of the lockfile catalog: **decision D2**.
- format-check from evidence: **decision D2**.
