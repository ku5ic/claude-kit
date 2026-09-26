# claude-kit

A Claude Code plugin: deterministic guard hooks, stack-aware context, a read-only investigation skill, `/audit` and `/write`, 22 stack pattern skills, and review agents. It leans on built-ins (`/plan`, `/code-review`, `/security-review`, Explore) and only adds what Claude Code lacks.

## Install

```sh
claude plugin marketplace add ku5ic/dotfiles
claude plugin install claude-kit@ku5ic
~/.claude/plugins/marketplaces/ku5ic/claude-kit/install-rules.sh
```

The last step links the always-on rules into `~/.claude/rules/claude-kit`; plugins cannot ship rules. Start from `templates/CLAUDE.md` for your own `~/.claude/CLAUDE.md`.

Requires bash 4+, `git`, `jq`, and `yq` (mikefarah). Formatters and linters (`prettier`, `shfmt`, `shellcheck`, `stylua`, `gitleaks`) are used when installed and skipped when not.

## What you get

| Part                      | What it does                                                                                                       |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| `hooks/`                  | Block risky shell and commit patterns, sanitize and format edited files, inject repo context, run checks on `Stop` |
| `skills/explore-patterns` | Read-only answer to a question or symptom; loads on its own, and in plan mode its findings feed the plan           |
| `skills/audit`            | `/audit a11y\|debt\|doc-drift\|perf\|verify`                                                                       |
| `skills/write`            | `/write commit\|pr\|release-notes\|devnote\|explainer\|review-comment\|review-reply\|stakeholder`                  |
| `skills/*-patterns`       | Stack knowledge packs, suggested per repo by `_stacks.yml`                                                         |
| `agents/`                 | `auditor`, `checker`, `debugger`, `plan-critic`, `researcher`, `tester`                                            |
| `rules/`                  | Output, evidence, change, workflow, tooling, and agent rules                                                       |
| `bin/`                    | Helpers on PATH: `run-checks.sh`, `scratch-dir.sh`, `git-base.sh`, and others                                      |

## Options

- `CLAUDE_GUARD_SKILLS=1` in your settings `env` blocks the first edit of a mapped file type until its pattern skill is loaded. Off by default.
- `CLAUDE_SANITIZE_TYPOGRAPHY=1` in your settings `env` makes the sanitizer rewrite em dashes, smart quotes, ellipses, and Unicode arrows in edited files to ASCII. Off by default; bidi control characters are always stripped.
- The `Stop` hook runs `bin/run-checks.sh` only when the turn edited files and left them uncommitted, and blocks once on failure.

## Develop

`bats tests/` runs the suite. `bin/bootstrap.sh` and `bin/doctor.sh` serve the symlinked layout in [ku5ic/dotfiles](https://github.com/ku5ic/dotfiles) and are not needed for a plugin install.
