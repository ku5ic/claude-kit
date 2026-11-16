# The kit CLI and status line

## How it's built

Every hook and helper is one Go binary (`bin/kit-<version>-<os>-<arch>`, darwin and linux, arm64 and amd64), run through the `bin/kit` launcher as `kit <subcommand>` or `kit hook <name>`. The first session start or `kit` command after an install or update downloads it from the matching GitHub release; until then the guards fail open.

## The kit CLI

Every hook runs through one `kit` binary, and the same binary answers the questions the hooks ask, so you can ask them too. `kit help` lists every command with its flags.

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
| See which accepted ADRs a file falls under                       | `kit adrs [path]`                                     |
| Check my overlay for unknown keys and wrong types                | `kit config --check`                                  |
| Scan a text for AI tells                                         | `kit slop <file>`                                     |
| Check two colours' contrast against WCAG                         | `kit contrast <fg> <bg>`                              |
| Prune old scratch files                                          | `kit scratch-rotate`                                  |

## In your own terminal

Claude Code puts `kit` on PATH only inside its sessions. To use it in your terminal, add to `~/.zshrc` (after `compinit`) or `~/.bashrc`:

```sh
export PATH="$HOME/.claude/plugins/marketplaces/ku5ic/bin:$PATH"
eval "$(kit completion zsh)"   # or: kit completion bash
```

## Status line

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
