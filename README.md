# claude-kit

**Guardrails and a working rhythm for Claude Code.** It stops the dangerous stuff, checks Claude's work before it says "done", and gives every session the context it would otherwise have to guess.

## The problem

Claude Code is capable. Left alone, it will also:

- run `rm -rf` against the wrong directory, or `git push --force` to main
- say "done" while a linter or test is still failing
- read your `.env` or SSH keys
- drop `out.log` and downloads into your repo root
- put an AI signature in your commit messages
- start every session knowing nothing about your stack or your check commands

Permission rules can't catch most of this. A prefix rule can't tell `git push` from `git push --force origin main`, it never sees a command hidden inside a pipeline or `$(...)`, and a plugin can't ship deny rules at all.

claude-kit closes those gaps with hooks: small, deterministic checks that run on every tool call, the same way in every repo.

## Install

```sh
claude plugin marketplace add ku5ic/claude-kit
claude plugin install claude-kit@ku5ic
~/.claude/plugins/marketplaces/ku5ic/install-rules.sh
```

The last line links the always-on rules into `~/.claude/rules/claude-kit` (plugins can't ship rules themselves). For your own `~/.claude/CLAUDE.md`, start from `templates/CLAUDE.md`.

Needs `git` and any bash, macOS or Linux. Formatters and linters are used when installed and skipped when not. Claude Code only: claude.ai and Cowork don't install a plugin with a top-level `bin/`.

## What changes after you install it

Nothing to learn up front. The hooks work on their own:

```text
session starts   -> Claude gets your stack, your check commands, and the skills that fit the repo
Claude runs bash -> dangerous commands are blocked, ambiguous ones ask you first
Claude edits     -> credential files are off limits; the file is formatted with your project's formatter
Claude commits   -> AI signatures and secrets in the staged diff are blocked
Claude says done -> your linters, type checkers, and tests run on the files it edited;
                    a failure sends it back to fix or report
```

- **Block** means Claude sees the reason and tries another way. You aren't interrupted.
- **Ask** means you get a normal permission prompt with the reason in it.

## The recommended flow

For anything bigger than a one-line fix:

1. **Ask first.** "How does X work?" or "why does Y break?" loads `investigate`, which reads the code and answers without editing anything.
2. **Plan.** Use the built-in `/plan`. In plan mode, `investigate`'s findings feed the plan. For a big plan, ask the `plan-critic` agent to check it against the real repo.
3. **Build one step at a time.** After each plan step Claude stops so you can review and commit. The Stop hook has already run the checks on what it touched.
4. **Review.** Run `/code-review` and `run-checks.sh` for the full suite. `/verify` runs the app and watches the change work.
5. **Ship.** `/write commit` and `/write pr` draft the message and description. Claude never commits or pushes without being asked.

Side trips when you need them:

- `/audit a11y|debt|doc-drift|perf` for a read-only audit report, and `/audit verify` to re-check one.
- `/deps` to merge Dependabot PRs and reconcile security alerts.

## Features

### Guards (always on)

| Guard | What it stops |
| --- | --- |
| **guard-bash** | Parses every command with a real bash parser, including pipelines, `$(...)`, subshells, and heredocs fed to a shell. Blocks `rm -rf ~`, force pushes, pushes to protected branches, `curl \| sh`, `eval`, `sh -c`, reading credential files, and writing shell rc files. Asks before `git push`, an unfrozen `pnpm install`, or a stray file written into the repo root |
| **guard-edit** | Blocks reading or writing `.env`, keys, and other credential files, and editing lockfiles, `.git/`, or shell rc files, whatever your permission rules say |
| **guard-commit** | Blocks AI signatures and AI-tell phrasing in commit messages, and secrets in the staged diff (runs `gitleaks` when installed) |
| **downloads** | `curl` and `wget` may write only into the gitignored `.claude/scratch/` |

### Quality (automatic)

| Hook | What it does |
| --- | --- |
| **Stop checks** | Runs your project's own linters, type checkers, and tests on just the files Claude edited, in parallel. Reuses the flags from your `package.json` scripts, Makefile, justfile, or CI. A linter blocks only on findings in lines Claude changed, so old warnings in a touched file don't trap it |
| **format** | Formats each edited file with the project's own formatter and config (Prettier, Biome, dprint, ruff, black, shfmt, stylua, gofmt). Never installs anything |
| **sanitize** | Strips invisible bidi characters (Trojan Source) from every written file |

### Context (automatic)

- **Session start:** stack, package manager, the check commands to use, the CLI tools on PATH, and which pattern skills to load.
- **Subagents:** every subagent gets the same repo context and its scratch path.

### Commands

| Command | What it does |
| --- | --- |
| `investigate` | Read-only answer to "how", "why", or "where". Loads on its own; never edits |
| `/audit <kind>` | `a11y`, `debt`, `doc-drift`, `perf`, or `verify`. Writes a report to scratch |
| `/write <kind>` | `commit`, `pr`, `release-notes`, `devnote`, `explainer`, `review-comment`, `review-reply`, `stakeholder` |
| `/deps` | Dependabot PRs and security alerts |
| `/meta <kind>` | Sharpen a prompt, refresh the pattern skills, draft a new skill, write a repo's conventions |

### Agents

| Agent | Use it to |
| --- | --- |
| `auditor` | Run a read-only audit for an `/audit` kind |
| `checker` | Get a one-line pass/fail from `run-checks.sh` |
| `debugger` | Find where and why something breaks. Hands the fix back |
| `plan-critic` | Attack a plan against the real repo before you approve it |
| `researcher` | Look up library docs and web pages, keeping network access out of code work |
| `tester` | Add or update tests for recent work. Never changes the code to make them pass |

In a plugin install, commands and agents are namespaced: `/claude-kit:audit`, `claude-kit:auditor`.

### Pattern skills

Stack knowledge Claude loads when the repo calls for it: React, Next.js App Router, Vue, Nuxt, TypeScript, JavaScript, Tailwind, Storybook, GraphQL, Python, Django, DRF, FastAPI, Go, Bash, Docker, OpenTofu, git, testing, security, logging, monitoring, backups, VPS provisioning, and WCAG 2.2.

### Rules (always on)

Six short files that shape how Claude works:

| Rule | In one line |
| --- | --- |
| `output` | Answer first, short by default, deliverables go to files |
| `evidence` | Never invent paths, APIs, versions, or test results; label how sure it is |
| `change` | Smallest fix for the defect, follow the existing pattern, no dead code |
| `workflow` | Never commit or push unasked; confirm before anything destructive |
| `tooling` | Reach for a CLI before reasoning; temporary files go to scratch |
| `agents` | When to spawn a subagent, and check its work before building on it |

## When a guard gets in your way

- **See why:** `kit explain bash '<cmd>'`, `kit explain edit <path>`, or `kit explain stop <file>` shows the decision and the exact rule. Nothing runs, blocks, or logs.
- **Turn one rule off:** add its slug to `disabled_rules` in `~/.claude/claude-kit.local.yml`. `kit explain` prints the slug, and `~/.claude/logs/guards.jsonl` logs it for every block.
- **A kit bug never blocks you:** a hook that crashes, or a missing binary, fails open.

## Configure

Defaults live in `kit.yml`. Your overrides go in `~/.claude/claude-kit.local.yml`, which merges on top (maps merge, lists append). Handy keys:

| Key | For |
| --- | --- |
| `protected_branches` | Branches Claude can't push to (default: `main`, `master`, `develop`, `production`, `release`) |
| `sensitive_paths` | Extra credential files to guard |
| `file_checks` | Your own Stop checks; a same-named entry replaces a built-in |
| `disabled_file_checks`, `disabled_formatters` | Turn a built-in check or formatter off |
| `check_timeout` | Seconds before a Stop check is killed and skipped (default 90) |

`kit config` prints the merged result, and `kit config --check` flags unknown keys and wrong types. `bin_lookups`, `test_script`, and `needs_files` are gone: package-manager environments (poetry, pipenv, Yarn PnP, bundler) are built in.

Two opt-in switches go in your settings `env`:

- `CLAUDE_GUARD_SKILLS=1` blocks the first edit of a file type until its pattern skill is loaded.
- `CLAUDE_SANITIZE_TYPOGRAPHY=1` also rewrites em dashes, smart quotes, and Unicode arrows to ASCII in edited files.

## How it's built

Every hook and helper runs in one Go binary (`bin/kit-<version>-<os>-<arch>`, darwin and linux, arm64 and amd64) behind same-name bash shims, in tens of milliseconds a call. The first call after an install or update downloads it from the matching GitHub release. `bin/doctor.sh` and `bin/bootstrap.sh` maintain the symlinked layout in [ku5ic/dotfiles](https://github.com/ku5ic/dotfiles), need bash 4.4+, `jq`, and mikefarah `yq`, and aren't needed for a plugin install.

## Develop

- Code: `go/`. Run `go test ./...`; end-to-end tests live in `go/e2e`.
- `bats tests/` covers the two bash scripts.
- `go/build.sh` builds the binaries for the `plugin.json` version. With `KIT_DEV=1`, `bin/kit` builds them itself and rebuilds when `go/` changes.
- Release: work lands on `dev`. Bump `.claude-plugin/plugin.json` `version`, then run the `release` workflow on `dev`. It publishes `v<version>` with the binaries and fast-forwards `main`, which is what installs track.
