# claude-kit

**Make Claude Code safe to leave alone.** One plugin that blocks destructive commands, runs your project's own checks before Claude says "done", and starts every session already knowing your stack.

```sh
claude plugin marketplace add ku5ic/claude-kit
claude plugin install kit@ku5ic
```

Zero config. Works in any repo on macOS and Linux. Every hook is a deterministic Go binary; the guards run in tens of milliseconds.

## Contents

- [What it fixes](#what-it-fixes)
- [Install](#install)
- [What you get](#what-you-get)
  - [Guards](#guards)
  - [Checks before "done"](#checks-before-done)
  - [Project-first tools](#project-first-tools)
  - [Formatting](#formatting)
  - [Context](#context)
  - [Rules](#rules)
  - [Commands](#commands)
  - [Agents](#agents)
  - [Pattern skills](#pattern-skills)
  - [Status line](#status-line)
  - [The kit CLI](#the-kit-cli)
- [The recommended flow](#the-recommended-flow)
- [Configure](#configure)
- [When a guard gets in your way](#when-a-guard-gets-in-your-way)
- [How it's built](#how-its-built)
- [Develop](#develop)

## What it fixes

| Without claude-kit, Claude will...                             | With it                                                                                                |
| -------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| run `rm -rf` on the wrong path, or `git push --force` to main  | Blocked. Every command is parsed, including pipelines, `$(...)`, and `sudo`/`env` wrappers             |
| say "done" while lint, types, or tests are failing             | Your checks run on the files it edited; a failure sends it back                                        |
| get stuck on old warnings in a file it barely touched          | Linters block only on lines Claude changed                                                             |
| not know your check commands, or guess them wrong              | Read from `package.json`, Makefile, justfile, cargo aliases, and your GitHub/GitLab CI                 |
| run a global `eslint` of the wrong version, or `npx` something | Tools resolve from the project first: `node_modules`, `.venv`, `go tool`, poetry, bundler. Never `npx` |
| read `.env` or `~/.ssh` keys                                   | Blocked, whatever your permission rules say                                                            |
| put an AI signature or a secret in a commit                    | Blocked at commit time, with `gitleaks` when installed                                                 |
| leave files unformatted, or formatted with the wrong config    | Each edit is formatted with your project's own formatter and config                                    |
| drop `out.log` and downloads in your repo root                 | Temporary files go to a gitignored `.claude/scratch/`                                                  |
| invent paths, APIs, versions, or test results                  | Rules require evidence and a confidence label                                                          |
| refactor half the repo for a one-line bug, or commit unasked   | Rules hold it to the smallest change and never commit without being asked                              |
| lose track of a plan halfway through                           | It's held to the next unchecked step and stops for your review after each                              |
| give generic advice that ignores your stack                    | Stack-specific pattern skills load when the repo calls for them                                        |
| spawn subagents that know none of the above                    | Every subagent gets the same rules and repo context                                                    |

Permission rules can't do most of this. A prefix rule can't tell `git push` from `git push --force origin main`, never sees a command inside a pipeline, and a plugin can't ship deny rules at all. Hooks can.

## Install

```sh
claude plugin marketplace add ku5ic/claude-kit
claude plugin install kit@ku5ic
```

- **Needs:** `git` and any bash, on macOS or Linux. Formatters and linters are used when installed and skipped when not; the kit never installs anything.
- **Claude Code only.** claude.ai and Cowork don't install a plugin with a top-level `bin/`.
- **Rules ship with the plugin.** Nothing to link. If an older install left `~/.claude/rules/claude-kit`, remove it; the session-start notice names it.
- **Your own CLAUDE.md:** start from `templates/CLAUDE.md`.

### Shell completion (optional)

Claude Code puts `kit` on PATH only inside its sessions. To use it in your terminal, add to `~/.zshrc` (after `compinit`) or `~/.bashrc`:

```sh
export PATH="$HOME/.claude/plugins/marketplaces/ku5ic/bin:$PATH"
eval "$(kit completion zsh)"   # or: kit completion bash
```

## What you get

Nothing to learn up front. After install:

```text
session starts   -> Claude gets the rules, your stack, your check commands, and the skills that fit
Claude runs bash -> dangerous commands are blocked, ambiguous ones ask you first
Claude edits     -> credential files are off limits; the file is formatted with your formatter
Claude commits   -> AI signatures and secrets in the staged diff are blocked
Claude says done -> your linters, type checkers, and tests run on what it edited
```

**Block** means Claude sees the reason and tries another way; you aren't interrupted. **Ask** means a normal permission prompt, with the reason in it.

### Guards

| Guard            | What it stops                                                                                                                                                                                                                                                                                          |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **guard-bash**   | Parses every command with a real bash parser and sees through `sudo`, `doas`, `env`. Blocks `rm -rf ~`, force pushes, pushes to protected branches, `curl \| sh`, `eval`, `sh -c`, credential reads, rc-file writes. Asks before `git push`, an unfrozen install, or a file written into the repo root |
| **guard-edit**   | Blocks reading or writing `.env`, keys, and credential files, and editing lockfiles, `.git/`, or shell rc files. Asks before editing a CI workflow                                                                                                                                                     |
| **guard-commit** | Blocks AI signatures and AI-tell phrasing in commit messages, and secrets in the staged diff                                                                                                                                                                                                           |
| **downloads**    | `curl` and `wget` may write only into `.claude/scratch/`                                                                                                                                                                                                                                               |
| **sanitize**     | Strips invisible bidi characters (Trojan Source) from every written file                                                                                                                                                                                                                               |

### Checks before "done"

When Claude finishes a turn, the Stop hook runs your linters, type checkers, and tests on just the files it edited, in parallel. A failure sends Claude back to fix it or report it.

- **Built in:** eslint, biome, stylelint, vitest, jest, ruff, mypy, pyright, golangci-lint, go vet, rubocop, markdownlint, yamllint, tofu fmt, shellcheck. Each runs only where the project configures it.
- **Changed lines only.** A linter blocks on findings in lines Claude changed, so old warnings don't trap it.
- **Your flags.** It reuses the flags from your own scripts.

For the whole project, `kit run-checks` runs every quality gate the repo declares, in every subproject. It finds them without config:

- **Tasks:** `package.json` scripts, Makefile, justfile, and cargo aliases, matched to five slots: `typecheck`, `lint`, `format-check`, `test`, `deadcode`. An odd-named task counts when its body runs a known tool.
- **Aggregates:** a `ci` script that chains `lint && test` runs each gate once, not twice.
- **CI:** gates from GitHub Actions and GitLab CI, run only with tools the project has. `kit.yml` `gate_discovery` says which jobs and steps are read.
- **Dead code:** knip, vulture, deadcode, and tflint findings block only on lines changed against the base branch.
- **Monorepos:** turbo and nx run affected packages once through the orchestrator.

`kit run-checks --plan` shows what would run, and why, without running it.

- **On review:** at the end of every `/code-review`, the reviewer runs `kit run-checks` in the checkout it reviewed and opens its report with the result; a failing gate becomes a finding once the reviewer has verified its cause. How a PR or another branch is checked out: `rules/workflow.md` section 1.
- **At the end of a plan:** ticking its last step without a `/code-review` since the last code edit blocks the stop until one runs.

### Project-first tools

Every tool the kit runs takes its binary from the project before anything else: `node_modules/.bin`, `.venv`, a `go.mod` tool directive, poetry, pipenv, Yarn PnP, bundler, or a version pinned by asdf or mise. In a JS workspace, the installed version is checked against the declaring package's range.

A tool the project declares but hasn't installed is skipped with the install command to run. It never falls back to a random global copy, and never to `npx`, `pnpm dlx`, or `uv run`, which can install packages.

### Formatting

Each edited file is formatted with the project's own formatter and config: Prettier (or prettierd), Biome, dprint, ruff, black, shfmt, stylua, gofmt. Two formatters claiming one file (a migration mid-way) means neither runs. In a repo with no Markdown formatter config, edited Markdown still gets formatted by the first of prettierd, prettier, deno, or mdformat found on PATH.

### Context

- **Session start:** the rules, stack, package manager, key package versions, the check commands, the CLI tools on PATH, and which pattern skills to load.
- **Subagents:** the same rules, repo context, and scratch path.
- **Plans:** in plan mode, Claude is pointed at `investigate`. Once a plan is approved, every prompt holds it to the next unchecked step.

### Rules

Seven short files, injected into every session and every subagent:

| Rule        | In one line                                                               |
| ----------- | ------------------------------------------------------------------------- |
| `output`    | Answer first, short by default, deliverables go to files                  |
| `evidence`  | Never invent paths, APIs, versions, or test results; label how sure it is |
| `change`    | Smallest fix for the defect, follow the existing pattern, no dead code    |
| `workflow`  | Never commit or push unasked; confirm before anything destructive         |
| `tooling`   | Reach for a CLI before reasoning; temporary files go to scratch           |
| `subagents` | When to spawn a subagent, and check its work before building on it        |
| `verify`    | What has to happen before a change is called done                         |

### Commands

| Command         | What it does                                                                                                  |
| --------------- | ------------------------------------------------------------------------------------------------------------- |
| `investigate`   | Read-only answer to "how", "why", or "where". Loads on its own; never edits                                   |
| `/audit <kind>` | `a11y`, `debt`, `doc-drift`, `perf`, or `verify` a prior report. Writes a report to scratch                   |
| `/write <kind>` | `commit`, `pr`, `release-notes`, `devnote`, `explainer`, `review-comment`, `review-reply`, `stakeholder`      |
| `/deps`         | Merge Dependabot PRs and reconcile security alerts                                                            |
| `/meta <kind>`  | `prompt` sharpening, `refresh` the pattern skills, draft a new `skill`, write a repo's `conventions`, `retro` |

Commands work bare (`/audit`) or namespaced (`/kit:audit`).

### Agents

| Agent             | Use it to                                                                     |
| ----------------- | ----------------------------------------------------------------------------- |
| `kit:auditor`     | Run a read-only audit for an `/audit` kind                                    |
| `kit:checker`     | Get a one-line pass/fail from `kit run-checks`                                |
| `kit:debugger`    | Find where and why something breaks. Hands the fix back                       |
| `kit:plan-critic` | Attack a plan against the real repo before you approve it                     |
| `kit:researcher`  | Look up library docs and web pages, keeping network access out of code work   |
| `kit:tester`      | Add or update tests for recent work. Never changes the code to make them pass |

### Pattern skills

Stack knowledge Claude loads when the repo calls for it: React, Next.js App Router, Vue, Nuxt, TypeScript, JavaScript, Tailwind, Storybook, GraphQL, Python, Django, DRF, FastAPI, Go, Bash, Docker, OpenTofu/Terraform, git, testing, security, logging, monitoring, backups, VPS provisioning, WCAG 2.2, and engineering fundamentals.

Set `CLAUDE_GUARD_SKILLS=1` to block the first edit of a file type until its skill is loaded.

### Status line

Two rows: model, agent, directory, and git state; then context used, cost, duration, effort, and rate limit. Subagents get a line each. Add to `~/.claude/settings.json`:

```json
"statusLine": { "type": "command", "command": "~/.claude/plugins/marketplaces/ku5ic/bin/kit statusline" },
"subagentStatusLine": { "type": "command", "command": "~/.claude/plugins/marketplaces/ku5ic/bin/kit subagent-statusline" }
```

### The kit CLI

| Command                                                 | What it does                                                    |
| ------------------------------------------------------- | --------------------------------------------------------------- |
| `kit run-checks [--plan]`                               | Every quality gate, in every subproject; `--plan` lists them    |
| `kit explain bash\|edit\|stop`                          | Why a guard or the Stop hook decides what it does. Runs nothing |
| `kit blast-radius <file> [symbol]`                      | Which files import it, tests and source counted apart           |
| `kit detect-stack`, `kit tasks`, `kit subprojects`      | What the kit sees in this repo                                  |
| `kit a11y-check <url>`                                  | A digest of axe violations on a running page                    |
| `kit config [--check]`                                  | The merged config, or only its warnings                         |
| `kit scratch-rotate [days]`, `kit skills-report [days]` | Prune old scratch and logs; skill usage telemetry               |

## The recommended flow

For anything bigger than a one-line fix:

1. **Ask.** "How does X work?" loads `investigate`, which reads the code and edits nothing.
2. **Plan** with the built-in `/plan`. For a big one, have `kit:plan-critic` check it against the repo.
3. **Build one step at a time.** Claude stops after each step so you can review and commit. The Stop hook has already checked what it touched.
4. **Review** at the end: `/code-review`, which ends by running `kit run-checks`, then `/verify` to watch the change work.
5. **Ship** with `/write commit` and `/write pr`. Claude never commits or pushes unasked.

Side trips: `/audit` for a read-only report, `/simplify <path>` to cut over-engineering, `/deps` for dependency PRs.

## Configure

Defaults live in `kit.yml`. Your overrides go in `~/.claude/claude-kit.local.yml`, merged on top: maps merge, lists append, and an entry in `checks`, `toolchain_checks`, or `formatters` named like a default updates that default's fields.

| Key                                            | For                                                                                          |
| ---------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `protected_branches`                           | Branches Claude can't push to (default `main`, `master`, `develop`, `production`, `release`) |
| `sensitive_paths`                              | Extra credential files to guard                                                              |
| `disabled_rules`                               | Guard rules to let through, by the slug `kit explain` prints                                 |
| `file_checks`, `disabled_file_checks`          | Add your own Stop checks, or turn a built-in off                                             |
| `disabled_checks`, `disabled_toolchain_checks` | Turn a `run-checks` gate off, by slot or by the label it prints                              |
| `disabled_task_providers`                      | Stop reading a task source (`make`, `package-scripts`, ...) at all                           |
| `disabled_formatters`                          | Turn a formatter off                                                                         |
| `check_timeout`                                | Seconds before a Stop check is skipped (default 90)                                          |
| `tool_resolution.path_fallback`                | Tools the project doesn't declare that may still run from PATH                               |

`kit config` prints the merged result; `kit config --check` flags unknown keys and wrong types.

Opt-in switches, in your settings `env`:

- `CLAUDE_GUARD_SKILLS=1`: block the first edit of a file type until its pattern skill is loaded.
- `CLAUDE_SANITIZE_TYPOGRAPHY=1`: rewrite em dashes, smart quotes, and Unicode arrows to ASCII in edited files.

## When a guard gets in your way

- **See why:** `kit explain bash '<cmd>'`, `kit explain edit <path>`, or `kit explain stop <file>` shows the decision and the exact rule. Nothing runs, blocks, or logs.
- **Turn one rule off:** add its slug to `disabled_rules`. Every block is logged with its slug in `~/.claude/logs/guards.jsonl`.
- **A kit bug never blocks you.** A crashing hook or a missing binary fails open. A missing binary is reported at session start; every other fail-open is logged.
- **A bad overlay never turns guards off.** If `claude-kit.local.yml` doesn't merge, the kit uses `kit.yml` alone and says why at session start.

## How it's built

Every hook and helper is one Go binary (`bin/kit-<version>-<os>-<arch>`, darwin and linux, arm64 and amd64), run through the `bin/kit` launcher as `kit <subcommand>` or `kit hook <name>`. The first session start or `kit` command after an install or update downloads it from the matching GitHub release; until then the guards fail open.

## Develop

- Guidance: `CLAUDE.md` maps every rule topic to the one file that owns it. Change guidance there or nowhere.
- Code: `go/`. Run `go test ./...`; end-to-end tests live in `go/e2e`, and `go/e2e/kit_repo_test.go` checks the kit's own files agree (skill map, skill and agent frontmatter, `kit.yml` keys).
- Behavior: `evals/` checks that the rules shape replies (answer first, ask before guessing, no unasked commits, artifacts in scratch). Run `claude plugin eval . --ablation none --scaffold --allow-tools Edit Write Bash --runs 5` before and after a rules change and compare the scores. About $10 a run.
- `go/build.sh` builds the binaries for the `plugin.json` version. With `KIT_DEV=1`, `bin/kit` builds them itself and rebuilds when `go/` changes.
- Release: work happens on feature branches off `main`, squash-merged by PR. A PR that bumps `.claude-plugin/plugin.json` `version` releases: on its merge, the `release` workflow publishes `v<version>` with the binaries. Installs take hooks, binaries, and rules only from a version bump.
