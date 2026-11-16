# The kit CLI, status line, and dependencies

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

## Dependencies

Required: `git` and bash. Every tool below is optional. A missing one, except `glab`, is named on the session-start `<tooling>` block's `missing:` line, and Claude leaves it unused.

| Tool         | Used by                                                                                                               | Without it                                                                      |
| ------------ | --------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| `gh`         | `/pr-*`, `/deps`, `/code-history`, `/code-challenge`, `/code-migrate` across an org, `/write-tickets`, linked tickets | `/pr-*` and `/deps` can't reach GitHub; `/code-history` works from commits only |
| `glab`       | `/pr-*`, `/deps`, `/code-history`, `/code-challenge`, review checkout, on GitLab                                      | the host's MCP connector, else `/code-history` works from commits only          |
| `rg`         | text search everywhere, `/audit-*`, `/code-*`, the gatherer agent                                                     | search uses grep                                                                |
| `sg`         | `kit code-graph imports` (`/code-visual`, `/code-impact`), `/code-migrate`, `/code-contract`, searches by shape       | `kit code-graph imports` fails; `rg` stands in for shape search                 |
| `fd`         | file search, `/audit-*`, `/code-*`                                                                                    | file search uses find                                                           |
| `tokei`      | `/audit-debt`, `/code-visual`                                                                                         | file sizes come from reading files                                              |
| `scc`        | `/code-visual` hotspots                                                                                               | the hotspot quadrant is skipped                                                 |
| `mmdc`       | `/code-visual` diagram rendering                                                                                      | diagrams go unrendered, and the report says so                                  |
| `gitleaks`   | the commit guard's staged-diff scan, `/audit-security`                                                                | the commit scan is skipped                                                      |
| `trufflehog` | `/audit-security` history scan                                                                                        | history goes unscanned                                                          |
| `actionlint` | `/audit-ci`                                                                                                           | `/audit-ci` skips its actionlint step                                           |
| `hyperfine`  | `/audit-perf` timings                                                                                                 | `/audit-perf` makes no timing claim                                             |
| `difft`      | `/code-history`, dropping reformat-only commits                                                                       | reformat-only commits stay in the ranking                                       |
| `adr`        | `/code-challenge` reading ADRs                                                                                        | `/code-challenge` lists the ADR folder's files                                  |
| `jq`         | JSON questions, `/code-visual`, `/deps`, `/pr-*`                                                                      | Claude reads the JSON instead                                                   |
| `gron`       | grepping flat JSON paths                                                                                              | `jq` stands in                                                                  |
| `yq`         | YAML and TOML questions                                                                                               | Claude reads the file instead                                                   |
| `qsv`        | CSV and TSV questions                                                                                                 | Claude reads the file instead                                                   |
| `git-absorb` | fixup commits from staged hunks                                                                                       | `git rebase -i` with fixups instead                                             |
