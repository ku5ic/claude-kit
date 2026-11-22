<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/claude-kit-lockup-dark-bg.svg">
  <img src="docs/assets/claude-kit-lockup-light-bg.svg" alt="claude-kit: ship with confidence" width="480">
</picture>

# claude-kit

**Ship with confidence.**

Claude Code treats your rules as advice. A rule in CLAUDE.md, a skill, or a prompt is something it can skip, and you find out in review. claude-kit runs those rules as hooks on every command, edit, plan, subagent report, and stop, and sends Claude back when it breaks one. It has to back its claims, pass your project's own checks before it says "done", and stop for your review after every step.

It also has 47 commands for the work around the code: audits, ADRs and RFCs, reviews of your PRs and other people's, your review queue, dependency PRs, incident retros and runbooks, and launch copy ([every command](docs/commands.md)). Each one detects your stack, your code host, and the tools you've connected, and asks when it can't tell. An audit or code report a subagent writes goes through the same claim check.

```sh
claude plugin marketplace add ku5ic/claude-kit
claude plugin install kit@ku5ic
```

Works in any repo on macOS and Linux with the defaults in `kit.yml`. The common dials are rows in Claude Code's `/config`, and every other override goes in `~/.claude/claude-kit.local.yml` ([configuration](docs/configuration.md)). It's built for one engineer. There's no team policy layer and no Windows support.

**It isn't an autopilot.** You approve the plan, review each step, and commit. The kit makes each review shorter, because what reaches you has already been checked.

## How a session runs

```mermaid
flowchart LR
    ask([You ask]):::you --> tool[Claude calls a tool]:::claude
    tool --> guards{Guards}:::kit
    guards -->|pass| runs[Tool runs,<br/>files formatted]:::claude
    guards -->|ask| decide([You decide]):::you
    decide -->|yes| runs
    guards -.->|block, with the reason| tool
    runs --> checks{Your checks<br/>at turn end}:::kit
    checks -.->|fail| tool
    checks -->|pass| review([You review<br/>and commit]):::you

    classDef you fill:#F7E9DF,stroke:#A9643A,stroke-width:1.5px,color:#1F3A3D
    classDef claude fill:#EEF3F2,stroke:#4A6366,stroke-width:1.5px,color:#1F3A3D
    classDef kit fill:#1F3A3D,stroke:#7FA8A9,stroke-width:2px,color:#F4EFE6,font-weight:600
```

Nothing to learn up front. After install:

| When                  | The kit                                                                                                          |
| --------------------- | ---------------------------------------------------------------------------------------------------------------- |
| A session starts      | Loads the rules, your stack, your check commands, and the pattern skills that fit                                |
| Claude runs a command | Blocks the dangerous ones, asks about the ambiguous ones, checks any file it changed                             |
| Claude edits a file   | Keeps credential files off limits to every file tool, wants a reason for a skipped test, runs your formatter     |
| Claude plans          | Has plan-critic read it against the repo and checks its claims before you approve; a resubmit lists what changed |
| A step ends           | Hands you a review sheet: lockfiles, configs, markers, and unplanned files first                                 |
| A subagent reports    | Sends back AI tells, unconfirmed claims, and an over-long report before Claude reads it                          |
| Claude commits        | Blocks AI signatures and secrets; your commit-msg hooks check the message                                        |
| Claude says done      | Runs your git hooks' checks on what it edited                                                                    |
| Claude replies        | Holds it to a word target and reminds it to check its claims                                                     |

**Block** means Claude gets the reason and tries another way; you aren't interrupted. **Ask** is a normal permission prompt with the reason in it. Every guard, and which of the two it does: [docs/guards.md](docs/guards.md).

What Claude gets back when it tries `git push --force origin main`:

```text
kit bash: BLOCKED git push --force

do    use --force-with-lease if you must

rule  git-force-push
cmd   git push --force origin main
```

## What it catches

Each of these costs you a correction round when it gets through. The kit sends it back to Claude first.

| Without claude-kit, Claude will...                                            | With it                                                                                                                                                                                           |
| ----------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| state a path, an API, or a test result in a plan or report that it never read | A second model checks each claim against what Claude read this session and sends back the ones nothing confirms                                                                                   |
| cite a `file:line` that doesn't exist in a plan or a gatherer report          | Sent back with each bad citation, with no model run                                                                                                                                               |
| skip or focus a test, or silence a linter, to get to green                    | Blocked, unless the line says why (`kit: allow <reason>`)                                                                                                                                         |
| import or add a package that may not exist                                    | Blocked until your manifest declares it, or its registry has it                                                                                                                                   |
| say "done" while lint, types, or tests are failing                            | Your checks run on the files it edited; a failure sends it back                                                                                                                                   |
| ask you to approve a plan nobody checked                                      | Sent to run `kit:plan-critic` against the repo first                                                                                                                                              |
| fill files, commits, PRs, and posts with AI filler                            | Sent back with each line and its fix                                                                                                                                                              |
| use a design token your project never defines                                 | Blocked                                                                                                                                                                                           |
| run `rm -rf` on the wrong path, or `git push --force` to main                 | Blocked. Every command is parsed, including pipelines, `$(...)`, and `sudo`/`env` wrappers                                                                                                        |
| read `.env` or `~/.ssh` keys                                                  | Blocked through the file tools, a local file server's MCP tools, and every parsed command, whatever your permission rules say. [SECURITY.md](SECURITY.md#known-gaps) lists what parsing can't see |
| put an AI signature or a secret in a commit                                   | Blocked at commit time, with `gitleaks` when installed; your commit-msg hooks check the message                                                                                                   |
| get stuck on old warnings in a file it barely touched                         | Linters block only on lines Claude changed                                                                                                                                                        |
| not know your check commands, or guess them wrong                             | Read from your CI, git hooks, and task runners; a small model sorts what they leave open, once                                                                                                    |
| run a global `eslint` of the wrong version, or `npx` something                | Tools resolve from the project first: `node_modules`, `.venv`, `go tool`, poetry, bundler. Never `npx` a package the project hasn't installed                                                     |
| leave files unformatted, or formatted with the wrong config                   | Each edit is formatted by your project's own fixers and config                                                                                                                                    |
| drop `out.log` and downloads in your repo root                                | Temporary files go to `.claude/scratch/`, pruned once a day after 30 days                                                                                                                         |
| refactor half the repo for a one-line bug, or commit unasked                  | Rules hold it to the smallest change and never commit without being asked                                                                                                                         |
| lose track of a plan halfway through                                          | It's held to the next unchecked step and stops for your review after each                                                                                                                         |
| give generic advice that ignores your stack                                   | Stack-specific pattern skills load when the repo calls for them                                                                                                                                   |
| spawn subagents that know none of the above                                   | Every subagent gets the same rules and repo context                                                                                                                                               |

Permission rules alone can't catch these. A prefix rule can't tell `git push` from `git push --force origin main`, never sees a command inside a pipeline, and a plugin can't ship deny rules at all. Hooks can.

## What it costs

More per turn. The bash guard takes about 11 ms, but the claim checker is a Haiku run capped at $1 and 240 seconds (`kit.yml` `claims:`), and every block is a turn you wait through.

## How you work with it

For anything bigger than a one-line fix, the kit holds Claude to five stages. Each one removes a way a session usually goes wrong:

| Stage                | You                                       | Without it                                  | With the kit                                                                                                  |
| -------------------- | ----------------------------------------- | ------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| 1. Ask               | "How does X work?"                        | Claude starts editing when you only asked   | `investigate` loads, reads the code, and edits nothing                                                        |
| 2. Plan              | `/plan` (built in)                        | You approve a plan nobody checked           | `kit:plan-critic` reads it against the repo, and its claims are checked, before you're asked to approve       |
| 3. Review each step  | Approve the plan                          | Claude runs through the whole plan at once  | Claude stops after every step so you can review and commit. The Stop hook has already checked what it touched |
| 4. Review the change | `/code-review`, then `/verify` (built in) | "Done" while checks fail                    | `/code-review`'s hook runs the full gate; `/verify` watches the change work                                   |
| 5. Ship              | `/write-commit`, `/pr-description`        | Commits you didn't ask for, AI-sounding PRs | Messages and PR text in your project's style, checked for AI tells. Claude never commits or pushes unasked    |

Side trips: an `/audit-<kind>` for a read-only report, `/simplify` (built in) to cut over-engineering, `/deps` for dependency PRs.

## Requirements

Needs `git`, bash, and `curl`, on macOS or Linux. A fresh macOS asks for the Command Line Tools the first time `git` runs, and some Linux installs lack `git` or `curl`: [dependencies](docs/dependencies.md) says what each already has. The plugin runs in Claude Code only; claude.ai chat has [its own zip](docs/chat.md).

## Get the most out of it

The kit is opinionated about tools: it steers Claude to `rg`, `fd`, `ast-grep`, and `jq` over the stock ones, and most of them aren't preinstalled.

- **Install `ast-grep` and `jq`.** With `ast-grep`, Claude lists every file that uses the one it's about to change; `/meta-retro` needs `jq`. The [dependencies](docs/dependencies.md) page says what each other tool adds.
- **Log in the `claude` CLI.** Once per repo, Haiku sorts your CI and hook commands into checks and fixers, and reports and subagent posts get their claims checked.
- **Connect your code host.** `gh`, `glab`, or the host's MCP connector runs the `/pr-*` commands and `/deps`.
- **Gitignore `.claude/scratch/` and `.claude/plans/`**, globally or per repo. The kit never edits your ignore files.
- **Write your own CLAUDE.md:** copy `templates/CLAUDE.md` to `~/.claude/CLAUDE.md` and fill it in.

## Learn more

| Page                                           | What's in it                                                                                  |
| ---------------------------------------------- | --------------------------------------------------------------------------------------------- |
| [docs/commands.md](docs/commands.md)           | Every command and agent: what it does, when to use it                                         |
| [docs/skills.md](docs/skills.md)               | The skills Claude loads on its own for your stack, and when each loads                        |
| [docs/guards.md](docs/guards.md)               | Every guard, what it stops, whether it blocks or asks, and what to do when one is in your way |
| [docs/checks.md](docs/checks.md)               | Which of your project's checks run before "done", and how tools are resolved                  |
| [docs/rules.md](docs/rules.md)                 | The seven rule files, and why each says what it says                                          |
| [docs/configuration.md](docs/configuration.md) | Presets, every overlay key, and what's on by default                                          |
| [docs/dependencies.md](docs/dependencies.md)   | What to install, what a fresh macOS or Linux already has, and what each tool adds             |
| [docs/cli.md](docs/cli.md)                     | The `kit` CLI, how it's built, the status line, and shell completion                          |
| [docs/chat.md](docs/chat.md)                   | The smaller plugin for claude.ai chat                                                         |

## Develop

Setup, tests, and how a change gets released: [CONTRIBUTING.md](CONTRIBUTING.md).
