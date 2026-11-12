# Configuration

## On by default, and what you switch on

| Feature                                                                     | Default  | Needs, or how to change it                                                                        |
| --------------------------------------------------------------------------- | -------- | ------------------------------------------------------------------------------------------------- |
| Guards, checks before "done", formatting, context, rules                    | On       | Nothing. Each guard rule can be let through with `disabled_rules`                                 |
| Gap-fill: a small model sorts what your configs leave open                  | On       | The `claude` CLI, logged in. Haiku, capped at $0.50 a call, cached per repo                       |
| Claim checker                                                               | On       | For subagent reports, the `claude` CLI, logged in. Turn off with `disabled_rules: [claims]`       |
| plan-critic before a plan is approved                                       | On       | Turn off with `disabled_rules: [plan-critic]`                                                     |
| kit-rules and every `global_skills` skill loaded before the first tool call | On       | Turn off with `disabled_rules: [rules-gate]`                                                      |
| A finished plan held until `/code-review` ran after its last edit           | On       | Turn off with `disabled_rules: [plan-done]`                                                       |
| Claude stops for your review after each plan step                           | On       | `step_pause: plan` stops once at the plan's end, `off` never stops                                |
| Pattern skills required before the first edit of a file type                | Off      | `guard_skills: true`, or `CLAUDE_GUARD_SKILLS=1` in your settings `env`                           |
| Em dashes, smart quotes, and Unicode arrows rewritten to ASCII              | Off      | `sanitize_typography: true`, or `CLAUDE_SANITIZE_TYPOGRAPHY=1` in your settings `env`             |
| Status line                                                                 | Off      | Two lines in `~/.claude/settings.json` ([Status line](../README.md#status-line))                  |
| `kit` in your own terminal                                                  | Off      | PATH and completion in your shell rc ([Shell completion](../README.md#shell-completion-optional)) |
| claude.ai chat                                                              | Separate | Upload the release's chat zip ([Use it in claude.ai chat](../README.md#use-it-in-claudeai-chat))  |

## Your overlay

Defaults live in `kit.yml`. Your overrides go in `~/.claude/claude-kit.local.yml`, merged on top: maps merge, lists append, so an overlay adds to a default list and can't remove from it. The exceptions: `slop.block_tiers` and every argv list (`cmd`, `private_cmd`, `command`) are replaced, and a `slop.rules` entry replaces the default with its `id`. Every `~/.claude` path in these docs is `$CLAUDE_CONFIG_DIR` when that's set.

| Key                       | For                                                                                                                                                                                     |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `protected_branches`      | Branches Claude can't push to                                                                                                                                                           |
| `sensitive_paths`         | Extra credential files to guard                                                                                                                                                         |
| `global_installs`         | Regexes for global package installs guard-bash blocks                                                                                                                                   |
| `rc_files`                | Shell rc files guard-bash and guard-edit won't let Claude write                                                                                                                         |
| `lockfile_globs`          | Lockfiles guard-edit won't let Claude edit by hand                                                                                                                                      |
| `root_artifact_exts`      | Extensions of a new file at the repo root that get a prompt                                                                                                                             |
| `preset`                  | A preset to merge under your overlay ([Presets](#presets))                                                                                                                              |
| `step_pause`              | When Claude stops a plan for your review: `step` (after each), `plan` (at its end), or `off`                                                                                            |
| `guard_skills`            | Block the first edit of a file type until its pattern skill is loaded                                                                                                                   |
| `sanitize_typography`     | Rewrite em dashes, smart quotes, and Unicode arrows to ASCII in edited files                                                                                                            |
| `disabled_rules`          | Guard rules to let through, by the slug `kit explain` prints                                                                                                                            |
| `disabled_checks`         | Turn a `run-checks` gate off, by check kind or by the label it prints                                                                                                                   |
| `disabled_task_providers` | Stop reading a task source (`make`, `package-scripts`, ...) at all                                                                                                                      |
| `on_edit`                 | Format a file type the project's own fixers don't claim                                                                                                                                 |
| `gate_timeout`            | Seconds before a Stop check is skipped                                                                                                                                                  |
| `gate_env`                | `NAME=value` pairs every check runs with                                                                                                                                                |
| `gate_discovery`          | CI jobs and steps never run as checks: deny lists of commands, names, and actions                                                                                                       |
| `classifier`              | The command gap-fill runs instead of `claude`, as an argv list                                                                                                                          |
| `claims`                  | The claim checker's model, timeout, budget, extra read-only tools, or a command run instead of `claude`                                                                                 |
| `connectors`              | The verbs that tell an MCP tool that writes from one that reads, and the input fields that hold code                                                                                    |
| `weakened_checks`         | What counts as a skipped or focused test, a silenced linter, and an assertion                                                                                                           |
| `package_lookups`         | The registry lookup for each package manager's add command                                                                                                                              |
| `user_pins`               | Your own Brewfile or `.tool-versions`, pinning tools for every project                                                                                                                  |
| `reply_limits`            | Word ceilings per reply: `chat`, `explain` and its trigger words, `detail_triggers` that lift the cap, per-`/write` kind; 0 is off. `uncapped_commands`: slash commands with no ceiling |
| `size_budgets`            | Word budgets, fenced code excluded, for a plan, a subagent report, a scratch report, and a connector post. Over one, the text is sent back once; 0 is off                               |
| `skip_dirs`               | Directories a file walk never enters                                                                                                                                                    |
| `sanitize_skip`           | Files `sanitize-output` leaves alone                                                                                                                                                    |
| `slop`                    | The AI-tell lexicon `kit slop` scans with: rules by tier, the density word list and threshold, which tiers block, files to skip                                                         |
| `deny_flags`              | Flags no check may set (`--fix`, `--watch`, ...)                                                                                                                                        |
| `tools`                   | The CLIs session start reports as available or missing                                                                                                                                  |
| `chat_bundle`             | The skills and rules the claude.ai chat zip carries                                                                                                                                     |
| `log_max_lines`           | Line cap for the kit's JSONL logs                                                                                                                                                       |
| Skill maps                | `skill_file_map`, `dependency_skills`, `skill_triggers`, `global_skills`: which pattern skills a file or dependency calls for                                                           |

`kit config` prints the merged result, each key marked with the files that set it; `kit config --check` flags unknown keys and wrong types.

## Presets

A preset is a set of these keys under a name. Put `preset: <name>` in your overlay, and its file merges between `kit.yml` and your overlay, so your own keys still win. Each file in [`presets/`](../presets) is a few lines:

| Preset     | Sets                                                                                                                        |
| ---------- | --------------------------------------------------------------------------------------------------------------------------- |
| `light`    | `step_pause: plan`, `disabled_rules: [plan-critic, claims, skills-nudge]`: one stop per plan, fewer passes, no skill nudges |
| `standard` | Nothing: `kit.yml` as it is                                                                                                 |
| `strict`   | `guard_skills: true`, `sanitize_typography: true`: both opt-in guards on                                                    |

`CLAUDE_GUARD_SKILLS=1` and `CLAUDE_SANITIZE_TYPOGRAPHY=1` in your settings `env` still turn those two guards on.
