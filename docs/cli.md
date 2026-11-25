<img src="assets/claude-kit-mark.svg" alt="" width="40">

# The kit CLI and status line

## How it's built

Every command hook and helper is one Go binary (`bin/kit-<version>-<os>-<arch>`, darwin and linux, arm64 and amd64), run through the `bin/kit` launcher as `kit <subcommand>` or `kit hook <name>`. The first session start or `kit` command after an install or update downloads it from the matching GitHub release; until then the guards fail open.

## The kit CLI

Every command hook runs through one `kit` binary, and the same binary answers the questions the hooks ask, so you can ask them too. `kit help` lists every command with its flags.

| I want to                                                        | Run                                                   |
| ---------------------------------------------------------------- | ----------------------------------------------------- |
| Know why a guard blocked a command or an edit                    | `kit explain bash '<cmd>'`, `kit explain edit <path>` |
| Know which checks run on a file when Claude stops                | `kit explain stop <file>`                             |
| See every check the project declares, and where each comes from  | `kit run-checks --plan`                               |
| Run every quality gate on what changed                           | `kit run-checks`                                      |
| See how often each rule fired, and what the claim checker caught | `kit stats [--rule r] [--since days]`                 |
| See what my project's configs say to run, and when               | `kit enforce --list`                                  |
| Let a gate that changed files run again                          | `kit gates reset`                                     |
| Find the files that import a file before I change it             | `kit blast-radius <file>`                             |
| See which base branch a diff or the full gate is scoped to       | `kit git-base`                                        |
| Run a git, `gh`, or `glab` read that can't turn into a write     | `kit read git log ...`, `kit read gh pr view ...`     |
| See which accepted ADRs a file falls under                       | `kit adrs [path]`                                     |
| Check my overlay for unknown keys and wrong types                | `kit config --check`                                  |
| Scan a text for AI tells                                         | `kit slop <file>`                                     |
| Check two colours' contrast against WCAG                         | `kit contrast <fg> <bg>`                              |
| Prune old scratch files now, not at the next day's first session | `kit scratch-rotate`                                  |
| Empty every scratch folder, whatever the files' age              | `kit scratch-rotate 0`                                |
| Remove the kit's logs, caches, and scratch before uninstalling   | `kit clean`, then `kit clean --yes`                   |

On a stacked branch, name its parent once, with the setting `gh pr create` reads: `git config branch.<child>.gh-merge-base <parent>`. `kit git-base` and the checks then count only the child's changes. After the parent is squash-merged and the child rebased, remove it with `git config --unset branch.<child>.gh-merge-base`: while the parent branch still exists, the diff would be too wide.

## In your own terminal

Claude Code puts `kit` on PATH only inside its sessions. To use it in your terminal, add to `~/.zshrc` (after `compinit`) or `~/.bashrc`:

```sh
export PATH="$HOME/.claude/plugins/marketplaces/ku5ic/bin:$PATH"
eval "$(kit completion zsh)"   # or: kit completion bash
```

## Status line

What a session costs and how full its context is, without asking. Two rows: model, agent, directory, and git state; then context used, cost, duration, effort, and rate limit. Git state is cached for `STATUSLINE_CACHE_TTL` seconds (default 1). Add to `~/.claude/settings.json`:

```json
"statusLine": { "type": "command", "command": "~/.claude/plugins/marketplaces/ku5ic/bin/kit statusline" }
```

It looks like this, with colour:

```text
Opus 5.5  my-app  main +0 ~0
███░░░░░░░ 32%  $15.62   1h 26m   effort:high  5h:31%
```

## Subagent rows

The plugin draws each subagent's row in the agent list itself, with no setting: name with a (+N) count of the agents below it, status, model and effort, context used, time running, and the tool it's running now, or its latest progress note. While it's on, a subagent's tool call can't move to the background and shows no live output. Turn it off with the `subagent_rows` option in `/plugin` config. Claude Code 2.1.323 has no agent rows for a plugin to draw, so it shows its own.

```text
kit:plan-critic  running  opus-5-5 high  18%  2m4s  Read statusline.go
kit:gatherer  completed  haiku-5-5 low  7%  found 3 call sites
```

A `subagentStatusLine` setting is deprecated in Claude Code and the kit no longer answers it; remove it from `~/.claude/settings.json`.
