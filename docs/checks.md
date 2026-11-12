# Checks before "done"

The kit enforces your project's own rules, not its own list of tools. It reads what your project already runs, and asks gap-fill only about what those configs leave open.

When Claude finishes a turn, the Stop hook runs the checks your git hooks would run on a commit (pre-commit, lint-staged, lefthook) on just the files it edited, in parallel. A project with none gets the file-scoped form of its CI checks. A failure sends Claude back to fix it or report it.

- **Changed lines only.** A linter blocks on findings in lines Claude changed, so old warnings don't trap it.
- **Your commands, your flags.** Every command is one your project states; `kit explain stop <file>` shows which claim a file and the exact command.

For the whole project, `kit run-checks` runs every quality gate, in every subproject. The `/code-review` hook runs it for you; Claude itself never does:

- **CI first:** the check steps of GitHub Actions and GitLab CI, and the tasks they run. `kit.yml` `gate_discovery` says which jobs and steps are never read.
- **Then each kind CI leaves out:** lint, typecheck, format-check, test, dead code, and security, from turbo and nx, task runners (`package.json` scripts, Makefile, justfile, and more), and pre-commit; last, a check the project states only by a tool's config or its language's manifest (`go.mod` states `go vet`).
- **Aggregates:** a `ci` script that chains `lint && test` runs each gate once, not twice.
- **Overlaps:** a check whose run another check of the same kind, in the same directory, already does is skipped as `covered by` it, and runs after all when that check doesn't.
- **What changed only:** on a branch, a gate with an affected form runs it, and a lint or format check runs on the changed files git doesn't ignore; the rest run whole. `--all` runs every gate on the whole tree.
- **Dead code:** dead-code findings block only on lines changed against the base branch.
- **Hands off your files:** a check that changes files fails, is never run again until `kit gates reset`, and `kit gates reset --restore` puts the files back.

`kit run-checks --plan` shows what would run, where each comes from, and why, without running it.

`/code-review` ends by running it, and ticking a plan's last step without a review blocks the stop until one runs: `skills/kit-rules/reference/verify.md`.

## Project-first tools

Every tool the kit runs takes its binary from the project before anything else: a local install, a `go.mod` tool directive, the package manager's environment, or a version pinned by asdf, mise, or a Brewfile. In a JS workspace, the installed version is checked against the declaring package's range.

A tool the project declares but hasn't installed is skipped with the install command to run. It never falls back to a random global copy, and never to `npx`, `pnpm dlx`, or `uv run`, which can install packages.

## Formatting

Each edited file is formatted by the fixers your git hooks would run on it (pre-commit, lint-staged, lefthook), else by the one formatter your project's config or dependencies state. Two formatters claiming one file (a migration mid-way) means neither runs. A file nothing claims gets `kit.yml` `on_edit`: Markdown is formatted by the first formatter found on PATH.
