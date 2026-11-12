# claude-kit

**Claude Code that shows its work.**

Claude Code treats your rules as advice. A rule in CLAUDE.md, a skill, or a prompt is something it can skip, and you find out in review. claude-kit runs those rules as hooks on every command, edit, plan, subagent report, and stop, and sends Claude back when it breaks one. It has to back its claims, pass your project's own checks before it says "done", and stop for your review after every step.

```sh
claude plugin marketplace add ku5ic/claude-kit
claude plugin install kit@ku5ic
```

Works in any repo on macOS and Linux with the defaults in `kit.yml`. Your overrides go in `~/.claude/claude-kit.local.yml` ([Presets vs configuration](#presets-vs-configuration)).

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

    classDef you fill:#dcfce7,stroke:#16a34a,color:#14532d
    classDef claude fill:#dbeafe,stroke:#2563eb,color:#1e3a8a
    classDef kit fill:#fef3c7,stroke:#d97706,color:#78350f
```

Nothing to learn up front. After install:

| When                  | The kit                                                                                                          |
| --------------------- | ---------------------------------------------------------------------------------------------------------------- |
| A session starts      | Loads the rules, your stack, your check commands, and the pattern skills that fit                                |
| Claude runs a command | Blocks the dangerous ones, asks about the ambiguous ones, checks any file it changed                             |
| Claude edits a file   | Keeps credential files off limits, wants a reason for a skipped test, runs your formatter                        |
| Claude plans          | Has plan-critic read it against the repo and checks its claims before you approve; a resubmit lists what changed |
| A step ends           | Hands you a review sheet: lockfiles, configs, markers, and unplanned files first                                 |
| A subagent reports    | Sends back AI tells, unconfirmed claims, and an over-long report before Claude reads it                          |
| Claude commits        | Blocks AI signatures and secrets; your commit-msg hooks check the message                                        |
| Claude says done      | Runs your git hooks' checks on what it edited                                                                    |
| Claude replies        | Holds it to a word ceiling and reminds it to check its claims                                                    |

**Block** means Claude gets the reason and tries another way; you aren't interrupted. **Ask** is a normal permission prompt with the reason in it. Every guard, and which of the two it does: [docs/guards.md](docs/guards.md).

## Contents

- [Enforced, not advised](#enforced-not-advised)
- [What it catches](#what-it-catches)
- [Measured, not promised](#measured-not-promised)
- [What it costs](#what-it-costs)
- [How you work with it](#how-you-work-with-it)
  - [Commands](#commands)
  - [Agents](#agents)
- [The pieces](#the-pieces)
  - [Guards](#guards)
  - [Checks before "done"](#checks-before-done)
  - [Context](#context)
  - [Rules](#rules)
  - [Pattern skills](#pattern-skills)
  - [Status line](#status-line)
  - [The kit CLI](#the-kit-cli)
- [Presets vs configuration](#presets-vs-configuration)
- [Install](#install)
  - [Shell completion (optional)](#shell-completion-optional)
- [Use it in claude.ai chat](#use-it-in-claudeai-chat)
- [When a guard gets in your way](#when-a-guard-gets-in-your-way)
- [How it's built](#how-its-built)
- [Develop](#develop)

## Enforced, not advised

Claude Code's permission rules can't do this. A prefix rule can't tell `git push` from `git push --force origin main`, never sees a command inside a pipeline, and a plugin can't ship deny rules at all. Hooks can.

## What it catches

Each of these costs you a correction round when it gets through. The kit sends it back to Claude first.

| Without claude-kit, Claude will...                                            | With it                                                                                                         |
| ----------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| state a path, an API, or a test result in a plan or report that it never read | A second model checks each claim against what Claude read this session and sends back the ones nothing confirms |
| cite a `file:line` that doesn't exist in a plan or a gatherer report          | Sent back with each bad citation, with no model run                                                             |
| skip or focus a test, or silence a linter, to get to green                    | Blocked, unless the line says why (`kit: allow <reason>`)                                                       |
| import or add a package that may not exist                                    | Blocked until your manifest declares it, or its registry has it                                                 |
| say "done" while lint, types, or tests are failing                            | Your checks run on the files it edited; a failure sends it back                                                 |
| ask you to approve a plan nobody checked                                      | Sent to run `kit:plan-critic` against the repo first                                                            |
| fill files, commits, PRs, and posts with AI filler                            | Sent back with each line and its fix                                                                            |
| use a design token your project never defines                                 | Blocked                                                                                                         |
| run `rm -rf` on the wrong path, or `git push --force` to main                 | Blocked. Every command is parsed, including pipelines, `$(...)`, and `sudo`/`env` wrappers                      |
| read `.env` or `~/.ssh` keys                                                  | Blocked, whatever your permission rules say                                                                     |
| put an AI signature or a secret in a commit                                   | Blocked at commit time, with `gitleaks` when installed; your commit-msg hooks check the message                 |
| get stuck on old warnings in a file it barely touched                         | Linters block only on lines Claude changed                                                                      |
| not know your check commands, or guess them wrong                             | Read from your CI, git hooks, and task runners; a small model sorts what they leave open, once                  |
| run a global `eslint` of the wrong version, or `npx` something                | Tools resolve from the project first: `node_modules`, `.venv`, `go tool`, poetry, bundler. Never `npx`          |
| leave files unformatted, or formatted with the wrong config                   | Each edit is formatted by your project's own fixers and config                                                  |
| drop `out.log` and downloads in your repo root                                | Temporary files go to `.claude/scratch/`, pruned by `kit scratch-rotate`                                        |
| refactor half the repo for a one-line bug, or commit unasked                  | Rules hold it to the smallest change and never commit without being asked                                       |
| lose track of a plan halfway through                                          | It's held to the next unchecked step and stops for your review after each                                       |
| give generic advice that ignores your stack                                   | Stack-specific pattern skills load when the repo calls for them                                                 |
| spawn subagents that know none of the above                                   | Every subagent gets the same rules and repo context                                                             |

## Measured, not promised

Same model, same repo, same prompt, run with the kit and without it. Every run is scored, and every score is in `evals/scorecard/`.

| You say               | Plain Claude Code                 | With claude-kit                                         |
| --------------------- | --------------------------------- | ------------------------------------------------------- |
| "Make calc better."   | Edits on a guess, 6 runs out of 6 | Asks which "better" you mean, edits nothing, 6 out of 6 |
| "Commit this change." | Commits, 6 out of 6               | Shows the message and waits for your yes, 6 out of 6    |

Where plain Claude Code already gets it right (fixing a failing test, answering a question without touching code, keeping AI tells out of a README or a landing page), it still does with the kit on.

## What it costs

More per turn. The bash guard takes about 11 ms, but the claim checker is a Sonnet run capped at $1 and 240 seconds (`kit.yml` `claims:`), and every block is a turn you wait through.

## How you work with it

For anything bigger than a one-line fix, the kit holds Claude to five stages. Each one removes a way a session usually goes wrong:

| Stage                | You                                       | Without it                                  | With the kit                                                                                                  |
| -------------------- | ----------------------------------------- | ------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| 1. Ask               | "How does X work?"                        | Claude starts editing when you only asked   | `investigate` loads, reads the code, and edits nothing                                                        |
| 2. Plan              | `/plan` (built in)                        | You approve a plan nobody checked           | `kit:plan-critic` reads it against the repo, and its claims are checked, before you're asked to approve       |
| 3. Review each step  | Approve the plan                          | Claude runs through the whole plan at once  | Claude stops after every step so you can review and commit. The Stop hook has already checked what it touched |
| 4. Review the change | `/code-review`, then `/verify` (built in) | "Done" while checks fail                    | `/code-review`'s hook runs the full gate; `/verify` watches the change work                                   |
| 5. Ship              | `/write commit`, `/kit:pr description`    | Commits you didn't ask for, AI-sounding PRs | Messages and PR text in your project's style, checked for AI tells. Claude never commits or pushes unasked    |

Side trips: `/audit` for a read-only report, `/simplify` (built in) to cut over-engineering, `/deps` for dependency PRs.

### Commands

Each command is a procedure Claude follows step by step, so the same request gets the same shape of answer every time. They work bare (`/audit`) or namespaced (`/kit:audit`), except `/kit:pr`: Claude Code reserves `/pr`.

| When you need                                                                             | Run              | What it does                                                                                                        |
| ----------------------------------------------------------------------------------------- | ---------------- | ------------------------------------------------------------------------------------------------------------------- |
| To know how, why, or where something works                                                | `investigate`    | Read-only answer to "how", "why", or "where". Loads on its own; never edits                                         |
| A report on accessibility, tech debt, doc drift, or performance                           | `/audit <kind>`  | `a11y`, `debt`, `doc-drift`, `perf`, or `verify` a prior report. Writes a report to scratch                         |
| A commit message, release notes, a devnote, an explainer, or a stakeholder summary        | `/write <kind>`  | `commit`, `release-notes`, `devnote`, `explainer`, `stakeholder`                                                    |
| PR text: your description, a review of someone's PR, or a reply to your reviewer          | `/kit:pr <kind>` | Draft a `description`, a `review` of someone's PR, or a `reply` to your reviewer; `publish` posts an approved draft |
| Dependabot PRs merged and security alerts sorted                                          | `/deps`          | Merge Dependabot PRs and reconcile security alerts                                                                  |
| AI tells and filler out of a text                                                         | `/unslop`        | Rewrite AI tells and filler in files, a `diff`, or pasted text; to scratch unless told `in place`                   |
| A sharper prompt, refreshed pattern skills, a new skill, a repo's conventions, or a retro | `/meta <kind>`   | `prompt` sharpening, `refresh` the pattern skills, draft a new `skill`, write a repo's `conventions`, `retro`       |

### Agents

Each agent is a subagent with one job, the tools for it, and a boundary it keeps: the debugger hands the fix back instead of applying it, and the tester never changes the code to make a test pass. Claude starts one itself when a skill or hook calls for it, and you can ask for one by name.

| When you need                                      | Agent               | What it does                                                                                 |
| -------------------------------------------------- | ------------------- | -------------------------------------------------------------------------------------------- |
| An audit for an `/audit` kind                      | `kit:auditor`       | Runs the audit read-only and writes the report                                               |
| To find where and why something breaks             | `kit:debugger`      | Localizes the fault with at most one probe edit. Hands the fix back                          |
| One fact: precedent, tests, or history             | `kit:gatherer`      | Answers with `file:line` only, never opinions                                                |
| A plan attacked before you approve it              | `kit:plan-critic`   | Reads the plan against the real repo                                                         |
| A text's claims checked before anyone relies on it | `kit:claim-checker` | Reports each claim nothing confirms, with the check that would settle it. A hook asks for it |
| Library docs or a web page                         | `kit:researcher`    | Looks them up, keeping network access out of code work                                       |
| Tests for recent work                              | `kit:tester`        | Adds or updates them and runs them. Never changes the code to make them pass                 |

## The pieces

### Guards

A force push, a read of `.env`, a skipped test, a package that doesn't exist. Every guard, what it stops, and whether it blocks or asks: [docs/guards.md](docs/guards.md).

### Checks before "done"

Your project's own checks run on the files Claude edited before it may say done, with tools taken from the project first and each edit formatted by your formatter: [docs/checks.md](docs/checks.md).

### Context

Claude starts a session knowing nothing about your repo, so it guesses your check commands and which tools to use. The kit tells it up front:

- **Session start:** stack, package managers, the check commands, the CLI tools on PATH, and which pattern skills to load. Gap-fill runs in the background to answer what the project's configs leave open.
- **Subagents:** the same repo context and scratch path.
- **Plans:** in plan mode, Claude is pointed at `investigate`. Once a plan is approved, every prompt holds it to the next unchecked step.

### Rules

Hooks catch what a program can check. The rules cover the judgment calls a hook can't see: how long a reply is, whether a claim was read or guessed, how big a fix should be, when to ask. They're seven short files in the `kit-rules` skill, which every session and subagent loads before its first tool call; a hook denies every other tool until it does:

| Rule        | In one line                                                               |
| ----------- | ------------------------------------------------------------------------- |
| `output`    | Answer first, short by default, deliverables go to files                  |
| `evidence`  | Never invent paths, APIs, versions, or test results; label how sure it is |
| `change`    | Smallest fix for the defect, follow the existing pattern, no dead code    |
| `workflow`  | Never commit or push unasked; confirm before anything destructive         |
| `tooling`   | Reach for a CLI before reasoning; temporary files go to scratch           |
| `subagents` | When to spawn a subagent, and check its work before building on it        |
| `verify`    | What has to happen before a change is called done                         |

Why each rule says what it says, with the evals behind it: [docs/rules.md](docs/rules.md).

### Pattern skills

Without them, Claude gives generic advice that ignores your stack. Each is stack knowledge Claude loads when the repo calls for it: React, Next.js App Router, Vue, Nuxt, TypeScript, JavaScript, Tailwind, Storybook, GraphQL, Python, Django, DRF, FastAPI, Go, Bash, Docker, OpenTofu/Terraform, git, testing, security, logging, monitoring, backups, VPS provisioning, WCAG 2.2, and engineering fundamentals. `slop-patterns` teaches rewriting AI tells; the slop guards send Claude to it.

Session start names the ones your repo needs. The first read or edit of a file type names its skill again, and `guard_skills: true` makes it required before the first edit. The switches are in [docs/configuration.md](docs/configuration.md).

### Status line

What a session costs and how full its context is, without asking. Two rows: model, agent, directory, and git state; then context used, cost, duration, effort, and rate limit. Subagents get a line each. Git state is cached for `STATUSLINE_CACHE_TTL` seconds (default 1). Add to `~/.claude/settings.json`:

```json
"statusLine": { "type": "command", "command": "~/.claude/plugins/marketplaces/ku5ic/bin/kit statusline" },
"subagentStatusLine": { "type": "command", "command": "~/.claude/plugins/marketplaces/ku5ic/bin/kit subagent-statusline" }
```

It looks like this, with colour:

```text
Opus 5.5  my-app  main +0 ~0
███░░░░░░░ 32%  $15.62   1h 26m   effort:high  5h:31%
```

And a line for each subagent:

```text
kit:plan-critic [running]  claude-opus-5-5  effort:high  18%
kit:gatherer [completed]  claude-haiku-5-5  effort:low  7%
```

### The kit CLI

Every hook runs through one `kit` binary, and the same binary answers the questions the hooks ask, so you can ask them too. `kit help` lists every command with its flags.

| I want to                                                       | Run                                                   |
| --------------------------------------------------------------- | ----------------------------------------------------- |
| Know why a guard blocked a command or an edit                   | `kit explain bash '<cmd>'`, `kit explain edit <path>` |
| Know which checks run on a file when Claude stops               | `kit explain stop <file>`                             |
| See every check the project declares, and where each comes from | `kit run-checks --plan`                               |
| Run every quality gate on what changed                          | `kit run-checks`                                      |
| See what my project's configs say to run, and when              | `kit enforce --list`                                  |
| Let a gate that changed files run again                         | `kit gates reset`                                     |
| Find the files that import a file before I change it            | `kit blast-radius <file>`                             |
| Check my overlay for unknown keys and wrong types               | `kit config --check`                                  |
| Scan a text for AI tells                                        | `kit slop <file>`                                     |
| Check two colours' contrast against WCAG                        | `kit contrast <fg> <bg>`                              |
| Prune old scratch files                                         | `kit scratch-rotate`                                  |

## Presets vs configuration

Guards, checks, formatting, and the rules are on from install. There are two ways to change them:

- A **preset** changes the kit's stance in one line. Pick one when you want fewer stops, or a stricter session, everywhere.
- A **key** changes one behavior: a protected branch, a guard rule let through, a formatter for a file type. Your keys win over the preset's.

Both go in `~/.claude/claude-kit.local.yml`, merged over the defaults in `kit.yml`.

| You want                                                                                | Set                                                       |
| --------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| Fewer stops: one review per plan, no plan-critic or claim-check passes, no skill nudges | `preset: light`                                           |
| The kit as shipped                                                                      | Nothing, or `preset: standard`                            |
| Pattern skills required before the first edit of a file type, and ASCII typography      | `preset: strict`                                          |
| Claude never to push to a branch                                                        | `protected_branches: [release]`                           |
| One guard rule let through                                                              | `disabled_rules: [<slug>]`, the slug `kit explain` prints |
| One review at the end of a plan instead of after each step                              | `step_pause: plan`                                        |
| A check gate off                                                                        | `disabled_checks: [<kind or label>]`                      |
| A file type formatted that your project's own fixers don't claim                        | `on_edit`                                                 |
| More credential files guarded                                                           | `sensitive_paths`                                         |

`kit config` prints the merged result, each key marked with the file that set it, and `kit config --check` flags unknown keys and wrong types. What is on by default, every key, the merge rules, and what each preset sets: [docs/configuration.md](docs/configuration.md).

## Install

```sh
claude plugin marketplace add ku5ic/claude-kit
claude plugin install kit@ku5ic
```

- **Needs:** `git` and any bash, on macOS or Linux. Formatters and linters are used when installed and skipped when not; the kit never installs anything.
- **Gap-fill:** the `claude` CLI, logged in. Once per repo, and again when its configs change, Haiku sorts the commands your CI, hooks, and task runners hold into checks and fixers, capped at $0.50 a call; the answers are cached and verified against your files. Without it, those commands are skipped and named.
- **Claim checker:** the `kit:claim-checker` agent, which Claude runs where you can watch it before a scratch report or a post goes out; plan-critic covers a plan. Inside a subagent, which can't start an agent, the hook runs the same instructions through the `claude` CLI. Without it, or when a run fails, claims go unchecked and nothing blocks.
- **Claude Code only.** claude.ai and Cowork don't install a plugin with a top-level `bin/`. Chat has [its own zip](#use-it-in-claudeai-chat).
- **Gitignore** `.claude/scratch/` and `.claude/plans/`, globally or per repo. The kit never edits your ignore files.
- **Rules ship with the plugin.** Nothing to link. If an older install left `~/.claude/rules/claude-kit`, remove it; the session-start notice names it.
- **Your own CLAUDE.md:** copy `templates/CLAUDE.md` to `~/.claude/CLAUDE.md` and fill it in.

### Shell completion (optional)

Claude Code puts `kit` on PATH only inside its sessions. To use it in your terminal, add to `~/.zshrc` (after `compinit`) or `~/.bashrc`:

```sh
export PATH="$HOME/.claude/plugins/marketplaces/ku5ic/bin:$PATH"
eval "$(kit completion zsh)"   # or: kit completion bash
```

## Use it in claude.ai chat

Each release attaches `kit-chat-<version>.zip`, a smaller plugin for chat on the web, desktop, and mobile. Upload it under Customize > Plugins, with code execution on. Updates mean uploading the new zip; it replaces the old one.

- **What's in it:** the `kit-rules` skill, built from the output, evidence, and change rules, plus `investigate`, `write`, `unslop`, `slop-patterns`, and `engineering-fundamentals`. They work on what you paste; chat has no repo. `kit.yml` `chat_bundle:` lists them.
- **No guards.** Chat runs no hooks, so nothing blocks, formats, or checks. `unslop` scans with a bundled `kit`, and reads its lexicon by hand if that can't run.
- **The rules can be skipped.** `kit-rules` loads when the model takes a task for writing or coding, and nothing enforces that. Only your claude.ai preferences reach every chat.
- **Mobile has no `/` menu.** Skills load from what you ask for.

## When a guard gets in your way

- **See why:** `kit explain bash '<cmd>'`, `kit explain edit <path>`, or `kit explain stop <file>` shows the decision and the exact rule. Nothing runs, blocks, or logs.
- **Turn one rule off:** add its slug to `disabled_rules`. Every block is logged with its slug in `~/.claude/logs/guards.jsonl`.
- **A kit bug never blocks you.** A crashing hook or a missing binary fails open. A crashing hook is logged in `guards.jsonl` with its error. A missing binary can't log: it's reported only when a session starts without it, and a hook that finds it gone mid-session says so on stderr alone. The binary goes missing when the first download can't finish: offline, a read-only plugin directory, a link too slow for `bin/kit`'s download timeout, or a platform other than macOS or Linux on arm64 or amd64. Until it lands, every guard is off.
- **A bad overlay never turns guards off.** If `claude-kit.local.yml` doesn't merge, the kit uses `kit.yml` alone and says why at session start.

## How it's built

Every hook and helper is one Go binary (`bin/kit-<version>-<os>-<arch>`, darwin and linux, arm64 and amd64), run through the `bin/kit` launcher as `kit <subcommand>` or `kit hook <name>`. The first session start or `kit` command after an install or update downloads it from the matching GitHub release; until then the guards fail open.

## Develop

Setup, tests, evals, and how a change gets released: [CONTRIBUTING.md](CONTRIBUTING.md).
