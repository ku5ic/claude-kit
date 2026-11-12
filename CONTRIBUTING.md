# Contributing

Bugs and ideas go in a GitHub issue. A guard bypass is a security report, so follow [SECURITY.md](SECURITY.md) instead.

## Before you change anything

- **Find the owner.** `CLAUDE.md` maps every rule topic to the one file that owns it. Change a topic in its owner, or nowhere; every other file links to it.
- **Check the decisions.** `docs/adr/` records why things are the way they are. A change that contradicts an accepted ADR comes with a new ADR that supersedes or amends it, as `CLAUDE.md` describes.

## Set up

You need Go (the `go` directive in `go/go.mod`), `git`, and bash, on macOS or Linux.

```sh
go/build.sh          # once, and again after a version bump
export KIT_DEV=1     # bin/kit now builds from go/ and rebuilds when it changes
bin/kit help
```

`go/build.sh` builds the release binaries for the `plugin.json` version; the launcher tests need this platform's one. With `KIT_DEV=1`, `bin/kit` runs `bin/kit-dev-<os>-<arch>` instead, and the release binaries stay as `go/build.sh` left them. In this repo, call `bin/kit`, never a `kit` on PATH, which is the installed release.

## Test

- **Everything CI runs:** `KIT_DEV=1 bin/kit run-checks --all`, or without `--all` for what changed since `main`. It reads the steps from `.github/workflows/ci.yml`, which are golangci-lint, `go vet`, deadcode, `go test -race`, the chat bundle check, shellcheck, and `claude plugin validate`.
- **Go tests:** `go test ./...` in `go/`. End-to-end tests live in `go/e2e`, and `go/e2e/kit_repo_test.go` checks that the kit's own files agree.
- **Behavior:** `evals/` checks that the rules shape replies. It holds only cases whose score differs with and without the kit. Once per release, build this version's binary with `go/build.sh --host`, then run `PATH="$PWD/bin:$PATH" claude plugin eval . --ablation with-without --scaffold --allow-tools Edit Write Bash --runs 1 -j 4` and compare against `evals/scorecard/`. The PATH puts the checkout's `kit` ahead of an installed one.

## The chat bundle

The `release` workflow builds the claude.ai chat zip. `kit bundle chat --check` validates what would go in it, and CI runs it too, so a bundled skill can't gain a Claude Code dependency unnoticed. Mark up what differs in chat with the comment markers `go/internal/chatbundle` reads, since Claude Code reads that text too.

## Pull requests and releases

- Work on a feature branch off `main`. PRs are squash-merged.
- A PR that changes `go/` or `hooks/hooks.json`, tests aside, bumps `.claude-plugin/plugin.json` `version`, and CI fails it otherwise. A fresh install pairs `main`'s hooks with the released binary for the current version, so a change without a bump would run new hooks on an old binary.
- Merging a version bump releases `v<version>`. The `release` workflow publishes it with the binaries and the chat zip, and installs take hooks, binaries, and rules only from a version bump.
