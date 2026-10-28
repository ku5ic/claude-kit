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
| not know your check commands, or guess them wrong              | Read from your CI, git hooks, and task runners; a small model sorts what they leave open, once         |
| run a global `eslint` of the wrong version, or `npx` something | Tools resolve from the project first: `node_modules`, `.venv`, `go tool`, poetry, bundler. Never `npx` |
| read `.env` or `~/.ssh` keys                                   | Blocked, whatever your permission rules say                                                            |
| put an AI signature or a secret in a commit                    | Blocked at commit time, with `gitleaks` when installed; your commit-msg hooks check the message        |
| leave files unformatted, or formatted with the wrong config    | Each edit is formatted by your project's own fixers and config                                         |
| drop `out.log` and downloads in your repo root                 | Temporary files go to `.claude/scratch/`, pruned by `kit scratch-rotate`                               |
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
- **Gap-fill:** the `claude` CLI, logged in. Once per repo, and again when its configs change, Haiku sorts the commands your CI, hooks, and task runners hold into checks and fixers, capped at $0.50 a call; the answers are cached and verified against your files. Without it, those commands are skipped and named.
- **Claude Code only.** claude.ai and Cowork don't install a plugin with a top-level `bin/`.
- **Gitignore** `.claude/scratch/` and `.claude/plans/`, globally or per repo. The kit never edits your ignore files.
- **Rules ship with the plugin.** Nothing to link. If an older install left `~/.claude/rules/claude-kit`, remove it; the session-start notice names it.
- **Your own CLAUDE.md:** copy `templates/CLAUDE.md` to `~/.claude/CLAUDE.md` and fill it in.

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
Claude commits   -> AI signatures and secrets are blocked; your commit-msg hooks check the message
Claude says done -> your git hooks' checks run on what it edited
Claude replies   -> each prompt carries its `reply_limits` word ceiling; a reply that ran over is told so next turn
```

**Block** means Claude sees the reason and tries another way; you aren't interrupted. **Ask** means a normal permission prompt, with the reason in it.

### Guards

| Guard                                  | What it stops                                                                                                                                                                                                                                                                                                                                                        |
| -------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **guard-bash**                         | Parses every command with a real bash parser and sees through `sudo`, `doas`, `env`. Blocks `rm -rf ~`, force pushes, pushes to protected branches, `curl \| sh`, `eval`, `sh -c`, credential reads, rc-file writes, global installs, a package manager other than the lockfile's. Asks before `git push`, adding a dependency, or a file written into the repo root |
| **guard-edit**                         | Blocks reading, grepping, or writing `.env`, keys, and credential files, and editing lockfiles, `.git/`, or shell rc files. Asks before editing a CI workflow or the kit overlay, or writing a new report-like file (`root_artifact_exts`) at the repo root                                                                                                          |
| **guard-commit**                       | Blocks AI signatures and AI-tell phrasing in commit messages, and secrets in the staged diff. Runs your commit-msg checks (pre-commit, husky, lefthook, commitlint) on the message                                                                                                                                                                                   |
| **guard-bash** rule `download-to-repo` | `curl` and `wget` may write only into `.claude/scratch/`                                                                                                                                                                                                                                                                                                             |
| **sanitize-output**                    | Strips invisible bidi characters (Trojan Source) from every written file except those `sanitize_skip` names: translations, templates, snapshots, fixtures                                                                                                                                                                                                            |
| **guard-skills**                       | Off by default; see `CLAUDE_GUARD_SKILLS` under Configure                                                                                                                                                                                                                                                                                                            |

### Checks before "done"

The kit enforces your project's own rules, not its own list of tools. It reads what your project already runs, and asks gap-fill only about what those configs leave open.

When Claude finishes a turn, the Stop hook runs the checks your git hooks would run on a commit (pre-commit, lint-staged, lefthook) on just the files it edited, in parallel. A project with none gets the file-scoped form of its CI checks. A failure sends Claude back to fix it or report it.

- **Changed lines only.** A linter blocks on findings in lines Claude changed, so old warnings don't trap it.
- **Your commands, your flags.** Every command is one your project states; `kit explain stop <file>` shows which claim a file and the exact command.

For the whole project, `kit run-checks` runs every quality gate, in every subproject. The `/code-review` hook runs it for you; Claude itself never does:

- **CI first:** the check steps of GitHub Actions and GitLab CI, and the tasks they run. `kit.yml` `gate_discovery` says which jobs and steps are never read.
- **Then each kind CI leaves out:** lint, typecheck, format-check, test, dead code, and security, from turbo and nx, task runners (`package.json` scripts, Makefile, justfile, and more), and pre-commit; last, a check the project states only by a tool's config or its language's manifest (`go.mod` states `go vet`).
- **Aggregates:** a `ci` script that chains `lint && test` runs each gate once, not twice.
- **What changed only:** on a branch, a gate with an affected form runs it, and a lint or format check runs on the changed files git doesn't ignore; the rest run whole. `--all` runs every gate on the whole tree.
- **Dead code:** dead-code findings block only on lines changed against the base branch.
- **Hands off your files:** a check that changes files fails, is never run again until `kit gates reset`, and `kit gates reset --restore` puts the files back.

`kit run-checks --plan` shows what would run, where each comes from, and why, without running it.

`/code-review` ends by running it, and ticking a plan's last step without a review blocks the stop until one runs: `rules/verify.md` section 1.

### Project-first tools

Every tool the kit runs takes its binary from the project before anything else: a local install, a `go.mod` tool directive, the package manager's environment, or a version pinned by asdf, mise, or a Brewfile. In a JS workspace, the installed version is checked against the declaring package's range.

A tool the project declares but hasn't installed is skipped with the install command to run. It never falls back to a random global copy, and never to `npx`, `pnpm dlx`, or `uv run`, which can install packages.

### Formatting

Each edited file is formatted by the fixers your git hooks would run on it (pre-commit, lint-staged, lefthook), else by the one formatter your project's config or dependencies state. Two formatters claiming one file (a migration mid-way) means neither runs. A file nothing claims gets `kit.yml` `on_edit`: Markdown is formatted by the first formatter found on PATH.

### Context

- **Session start:** the rules, stack, package managers, the check commands, the CLI tools on PATH, and which pattern skills to load. Gap-fill runs in the background to answer what the project's configs leave open.
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
| `kit:debugger`    | Find where and why something breaks. Hands the fix back                       |
| `kit:plan-critic` | Attack a plan against the real repo before you approve it                     |
| `kit:researcher`  | Look up library docs and web pages, keeping network access out of code work   |
| `kit:tester`      | Add or update tests for recent work. Never changes the code to make them pass |

### Pattern skills

Stack knowledge Claude loads when the repo calls for it: React, Next.js App Router, Vue, Nuxt, TypeScript, JavaScript, Tailwind, Storybook, GraphQL, Python, Django, DRF, FastAPI, Go, Bash, Docker, OpenTofu/Terraform, git, testing, security, logging, monitoring, backups, VPS provisioning, WCAG 2.2, and engineering fundamentals.

`CLAUDE_GUARD_SKILLS=1` makes them required: see the switches under Configure.

### Status line

Two rows: model, agent, directory, and git state; then context used, cost, duration, effort, and rate limit. Subagents get a line each. Git state is cached for `STATUSLINE_CACHE_TTL` seconds (default 1). Add to `~/.claude/settings.json`:

```json
"statusLine": { "type": "command", "command": "~/.claude/plugins/marketplaces/ku5ic/bin/kit statusline" },
"subagentStatusLine": { "type": "command", "command": "~/.claude/plugins/marketplaces/ku5ic/bin/kit subagent-statusline" }
```

### The kit CLI

`kit help` lists every command with its flags. The ones you'll reach for: `kit run-checks`, `kit explain`, `kit blast-radius`, `kit enforce --list`, `kit gates reset`, and `kit config --check`.

## The recommended flow

For anything bigger than a one-line fix:

1. **Ask.** "How does X work?" loads `investigate`, which reads the code and edits nothing.
2. **Plan** with the built-in `/plan`. For a big one, have `kit:plan-critic` check it against the repo.
3. **Build one step at a time.** Claude stops after each step so you can review and commit. The Stop hook has already checked what it touched.
4. **Review** at the end: `/code-review`, whose hook runs the full gate, then `/verify` to watch the change work.
5. **Ship** with `/write commit` and `/write pr`. Claude never commits or pushes unasked.

Side trips: `/audit` for a read-only report, `/simplify <path>` to cut over-engineering, `/deps` for dependency PRs.

## Configure

Defaults live in `kit.yml`. Your overrides go in `~/.claude/claude-kit.local.yml`, merged on top: maps merge, lists append, so an overlay adds to a default list and can't remove from it. Every `~/.claude` path in this README is `$CLAUDE_CONFIG_DIR` when that's set.

| Key                       | For                                                                                                                                                                                     |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `protected_branches`      | Branches Claude can't push to                                                                                                                                                           |
| `sensitive_paths`         | Extra credential files to guard                                                                                                                                                         |
| `global_installs`         | Regexes for global package installs guard-bash blocks                                                                                                                                   |
| `rc_files`                | Shell rc files guard-bash and guard-edit won't let Claude write                                                                                                                         |
| `lockfile_globs`          | Lockfiles guard-edit won't let Claude edit by hand                                                                                                                                      |
| `root_artifact_exts`      | Extensions of a new file at the repo root that get a prompt                                                                                                                             |
| `disabled_rules`          | Guard rules to let through, by the slug `kit explain` prints                                                                                                                            |
| `disabled_checks`         | Turn a `run-checks` gate off, by check kind or by the label it prints                                                                                                                   |
| `disabled_task_providers` | Stop reading a task source (`make`, `package-scripts`, ...) at all                                                                                                                      |
| `on_edit`                 | Format a file type the project's own fixers don't claim                                                                                                                                 |
| `gate_timeout`            | Seconds before a Stop check is skipped                                                                                                                                                  |
| `gate_env`                | `NAME=value` pairs every check runs with                                                                                                                                                |
| `gate_discovery`          | CI jobs and steps never run as checks: deny lists of commands, names, and actions                                                                                                       |
| `classifier`              | The command gap-fill runs instead of `claude`, as an argv list                                                                                                                          |
| `user_pins`               | Your own Brewfile or `.tool-versions`, pinning tools for every project                                                                                                                  |
| `reply_limits`            | Word ceilings per reply: `chat`, `explain` and its trigger words, `detail_triggers` that lift the cap, per-`/write` kind; 0 is off. `uncapped_commands`: slash commands with no ceiling |
| `skip_dirs`               | Directories a file walk never enters                                                                                                                                                    |
| `sanitize_skip`           | Files `sanitize-output` leaves alone                                                                                                                                                    |
| `deny_flags`              | Flags no check may set (`--fix`, `--watch`, ...)                                                                                                                                        |
| `tools`                   | The CLIs session start reports as available or missing                                                                                                                                  |
| `log_max_lines`           | Line cap for the kit's JSONL logs                                                                                                                                                       |
| Skill maps                | `skill_file_map`, `dependency_skills`, `skill_triggers`, `global_skills`: which pattern skills a file or dependency calls for                                                           |

`kit config` prints the merged result; `kit config --check` flags unknown keys and wrong types.

Opt-in switches, in your settings `env`:

- `CLAUDE_GUARD_SKILLS=1`: block the first edit of a file type until its pattern skill is loaded.
- `CLAUDE_SANITIZE_TYPOGRAPHY=1`: rewrite em dashes, smart quotes, and Unicode arrows to ASCII in edited files.

## When a guard gets in your way

- **See why:** `kit explain bash '<cmd>'`, `kit explain edit <path>`, or `kit explain stop <file>` shows the decision and the exact rule. Nothing runs, blocks, or logs.
- **Turn one rule off:** add its slug to `disabled_rules`. Every block is logged with its slug in `~/.claude/logs/guards.jsonl`.
- **A kit bug never blocks you.** A crashing hook or a missing binary fails open. A missing binary is reported at session start; every other fail-open is logged. The binary goes missing when the first download can't finish: offline, a read-only plugin directory, a link too slow for `bin/kit`'s download timeout, or a platform other than macOS or Linux on arm64 or amd64. Until it lands, every guard is off.
- **A bad overlay never turns guards off.** If `claude-kit.local.yml` doesn't merge, the kit uses `kit.yml` alone and says why at session start.

## How it's built

Every hook and helper is one Go binary (`bin/kit-<version>-<os>-<arch>`, darwin and linux, arm64 and amd64), run through the `bin/kit` launcher as `kit <subcommand>` or `kit hook <name>`. The first session start or `kit` command after an install or update downloads it from the matching GitHub release; until then the guards fail open.

## Develop

- Guidance: `CLAUDE.md` maps every rule topic to the one file that owns it. Change a topic in its owner, or nowhere.
- Code: `go/`. Run `go/build.sh` once (the launcher tests need this platform's binary), then `go test ./...` in `go/`; end-to-end tests live in `go/e2e`, and `go/e2e/kit_repo_test.go` checks the kit's own files agree (skill map, skill and agent frontmatter, `kit.yml` keys).
- Behavior: `evals/` checks that the rules shape replies (answer first, ask before guessing, no unasked commits, artifacts in scratch). Run `claude plugin eval . --ablation none --scaffold --allow-tools Edit Write Bash --runs 5` before and after a rules change and compare the scores. About $10 a run.
- `go/build.sh` builds the binaries for the `plugin.json` version. With `KIT_DEV=1`, `bin/kit` builds them itself and rebuilds when `go/` changes.
- Release: work happens on feature branches off `main`, squash-merged by PR. A PR that bumps `.claude-plugin/plugin.json` `version` releases: on its merge, the `release` workflow publishes `v<version>` with the binaries. Installs take hooks, binaries, and rules only from a version bump.
