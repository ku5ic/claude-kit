<img src="assets/claude-kit-mark.svg" alt="" width="40">

# Configuration

The common dials are rows in Claude Code's `/config` ([From `/config`](#from-config)). Every other change goes in `~/.claude/claude-kit.local.yml`, merged over the defaults in `kit.yml`. `kit config` prints the merged result, each key marked with the file that set it, and `kit config --check` flags unknown keys and wrong types.

## Common changes

| You want                                                                                           | Set                                                       |
| -------------------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| Fewer stops: one review per plan, no plan-critic pass, no skill nudges                             | `preset: light`, or the Kit: preset row                   |
| The kit as shipped                                                                                 | Nothing, or `preset: standard`                            |
| Pattern skills required before the first edit of a file type, ASCII typography, and claims checked | `preset: strict`                                          |
| Claude never to push to a branch                                                                   | `protected_branches: [staging]`                           |
| One guard rule let through                                                                         | `disabled_rules: [<slug>]`, the slug `kit explain` prints |
| One review at the end of a plan instead of after each step                                         | `step_pause: plan`, or the Kit: step pause row            |
| Longer chat replies                                                                                | `reply_length: moderate`, or the Kit: reply length row    |
| A check gate off                                                                                   | `disabled_checks: [<kind or label>]`                      |
| A file type formatted that your project's own fixers don't claim                                   | `on_edit`                                                 |
| More credential files guarded                                                                      | `sensitive_paths`                                         |

## On by default, and what you switch on

| Feature                                                                                                                                                                                                                                | Default  | Needs, or how to change it                                                                                    |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- | ------------------------------------------------------------------------------------------------------------- |
| Guards, checks before "done", formatting, context, rules                                                                                                                                                                               | On       | Nothing. Each guard rule can be let through with `disabled_rules`                                             |
| Gap-fill: a small model sorts what your configs leave open                                                                                                                                                                             | On       | The `claude` CLI, logged in. Haiku, capped at $0.50 a call, cached per repo                                   |
| Claim checker                                                                                                                                                                                                                          | Off      | `preset: strict`, or `claims: {timeout: 240}`, and the `claude` CLI, logged in, for plans, reports, and posts |
| plan-critic before a plan is approved                                                                                                                                                                                                  | On       | Turn off with `disabled_rules: [plan-critic]`                                                                 |
| Scratch files past 30 days pruned once a day at session start                                                                                                                                                                          | On       | Turn off with `disabled_rules: [scratch-rotate]`                                                              |
| kit-rules and every `global_skills` skill loaded before the first tool call                                                                                                                                                            | On       | Turn off with `disabled_rules: [rules-gate]`                                                                  |
| A finished plan held until `/code-review` ran after its last edit                                                                                                                                                                      | On       | Turn off with `disabled_rules: [plan-done]`                                                                   |
| A `/code-review` report saved to scratch before Claude asks or stops                                                                                                                                                                   | On       | Turn off with `disabled_rules: [review-report]`                                                               |
| Code edits held after a `/code-review` until you pick which findings to fix                                                                                                                                                            | On       | Turn off with `disabled_rules: [review-pick]`                                                                 |
| Claude stops for your review after each plan step                                                                                                                                                                                      | On       | `step_pause: plan` stops once at the plan's end, `off` never stops                                            |
| Pattern skills required before the first edit of a file type                                                                                                                                                                           | Off      | `guard_skills: true`, or `CLAUDE_GUARD_SKILLS=1` in your settings `env`                                       |
| Em dashes, smart quotes, and Unicode arrows rewritten to ASCII                                                                                                                                                                         | Off      | `sanitize_typography: true`, or `CLAUDE_SANITIZE_TYPOGRAPHY=1` in your settings `env`                         |
| Status line                                                                                                                                                                                                                            | Off      | One line in `~/.claude/settings.json` ([Status line](cli.md#status-line))                                     |
| Subagent rows: status, model, effort, and the tool each agent is running                                                                                                                                                               | On       | Turn off with the `subagent_rows` option in `/plugin` config ([Subagent rows](cli.md#subagent-rows))          |
| A system-prompt section and a per-prompt reminder putting the kit's rules ahead of default and mode instructions. Where Claude Code skips the section, a prompt carries its text, the first one once the machine has recorded the skip | On       | Turn off with the `precedence` option in `/plugin` config                                                     |
| `/kit-stats`, which prints `kit stats` ([Commands](commands.md))                                                                                                                                                                       | On       | Turn off with the `kit_stats` option in `/plugin` config                                                      |
| `kit` in your own terminal                                                                                                                                                                                                             | Off      | PATH and completion in your shell rc ([Shell completion](cli.md#in-your-own-terminal))                        |
| claude.ai chat                                                                                                                                                                                                                         | Separate | Upload the release's chat zip ([Use it in claude.ai chat](chat.md))                                           |

## From `/config`

Eight settings are rows in Claude Code's `/config`, each titled `Kit:` and described there. Claude Code saves them under `pluginConfigs` in `~/.claude/settings.json`, and the kit reads that file for its hooks and the `kit` CLI alike, from the next call after a change. Managed settings aren't read.

| Row                        | Key             | Choices                                       |
| -------------------------- | --------------- | --------------------------------------------- |
| Kit: preset                | `preset`        | `default`, `standard`, `light`, `strict`      |
| Kit: step pause            | `step_pause`    | `default`, `step`, `plan`, `off`              |
| Kit: gate receipt          | `receipt`       | `default`, `all`, `failures`                  |
| Kit: reply length          | `reply_length`  | `default`, `terse`, `moderate`, `explanatory` |
| Kit: reply slack           | `reply_slack`   | `default`, `none`, `25%`, `50%`, `100%`       |
| Kit: rules take precedence | `precedence`    | on, off                                       |
| Kit: /kit-stats command    | `kit_stats`     | on, off                                       |
| Kit: subagent rows         | `subagent_rows` | on, off                                       |

The last three switch a mod on or off and have no overlay key. For the first five, `default` sets nothing, so your overlay, its preset, or `kit.yml` decides. Any other choice wins over your overlay, and a preset picked here replaces the overlay's `preset:`. The order is `kit.yml`, the preset, your overlay, then `/config`, and `kit config` names `settings.json` for each key a row set.

## Your overlay

Defaults live in `kit.yml`. Your overrides go in `~/.claude/claude-kit.local.yml`, merged on top, where maps merge and lists append. A quoted `"!x"` in a list removes the default `x`, and `kit config --check` warns when it removes nothing. The exceptions: `slop.block_tiers` and every argv list (`cmd`, `private_cmd`, `command`) are replaced, and a `slop.rules` entry replaces the default with its `id`. Every `~/.claude` path in these docs is `$CLAUDE_CONFIG_DIR` when that's set.

| Key                       | For                                                                                                                                                                                                                               |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `protected_branches`      | Branches Claude can't push to                                                                                                                                                                                                     |
| `sensitive_paths`         | Extra credential files to guard, or a quoted `"!x"` exception (`kit.yml` ships `!.env.example`); the last entry a path matches decides                                                                                            |
| `global_installs`         | Regexes for global package installs guard-bash blocks                                                                                                                                                                             |
| `rc_files`                | Shell rc files guard-bash and guard-edit won't let Claude write                                                                                                                                                                   |
| `lockfile_globs`          | Lockfiles guard-edit won't let Claude edit by hand                                                                                                                                                                                |
| `root_artifact_exts`      | Extensions of a new file at the repo root that get a prompt                                                                                                                                                                       |
| `preset`                  | A preset to merge under your overlay ([Presets](#presets))                                                                                                                                                                        |
| `step_pause`              | When Claude stops a plan for your review: `step` (after each), `plan` (at its end), or `off`                                                                                                                                      |
| `guard_skills`            | Block the first edit of a file type until its pattern skill is loaded                                                                                                                                                             |
| `sanitize_typography`     | Rewrite em dashes, smart quotes, and Unicode arrows to ASCII in edited files                                                                                                                                                      |
| `disabled_rules`          | Guard rules to let through, by the slug `kit explain` prints                                                                                                                                                                      |
| `shadow_rules`            | Guard rules to try first: each block or deny is logged as `shadow` in `guards.jsonl`, and the call goes through                                                                                                                   |
| `disabled_checks`         | Turn a `run-checks` gate off, by check kind or by the label it prints                                                                                                                                                             |
| `disabled_task_providers` | Stop reading a task source (`make`, `package-scripts`, ...) at all                                                                                                                                                                |
| `on_edit`                 | Format a file type the project's own fixers don't claim                                                                                                                                                                           |
| `receipt`                 | When Stop and the full gate print their receipt: `all`, or `failures` for only a failed or skipped gate                                                                                                                           |
| `reply_length`            | Which `reply_limits.modes` entry sets the chat and explain targets: `terse`, `moderate`, or `explanatory`                                                                                                                         |
| `reply_slack`             | How far past its target a reply or a `size_budgets` text may run before it's flagged or sent back: `none`, `25%`, `50%`, or `100%`                                                                                                |
| `attribution`             | The AI tool names and patterns guard-attribution blocks in git and GitHub text                                                                                                                                                    |
| `gate_timeout`            | Seconds before a Stop check is skipped                                                                                                                                                                                            |
| `architecture_dir`        | Where `/code-visual` keeps each repo's architecture model, read by `kit code-graph model-dir`                                                                                                                                     |
| `gate_env`                | `NAME=value` pairs every check runs with                                                                                                                                                                                          |
| `gate_discovery`          | CI jobs and steps never run as checks: deny lists of commands, names, and actions                                                                                                                                                 |
| `classifier`              | The command gap-fill runs instead of `claude`, as an argv list                                                                                                                                                                    |
| `claims`                  | The claim checker's model, timeout, budget, extra read-only tools, or a command run instead of `claude`. It runs only with a `timeout` above 0, as the strict preset sets                                                         |
| `connectors`              | The verbs that tell an MCP tool that writes from one that reads, the input fields that hold code, and the ones that hold a path (`path_fields`), which get guard-edit's path rules on the local file servers `path_servers` names |
| `weakened_checks`         | What counts as a skipped or focused test, a silenced linter, an assertion, a snapshot, and a snapshot update                                                                                                                      |
| `package_lookups`         | The registry lookup for each package manager's add command                                                                                                                                                                        |
| `user_pins`               | Your own Brewfile or `.tool-versions`, pinning tools for every project                                                                                                                                                            |
| `reply_limits`            | Word targets per reply: `modes` (each mode's `chat` and `explain`), the explain trigger words, `detail_triggers` that lift the target, per `write-*` skill; 0 is off. `uncapped_commands`: slash commands with no target          |
| `size_budgets`            | Word budgets, fenced code excluded, for a plan, a subagent report, a scratch report, and a connector post. Past one plus `reply_slack`, the text is sent back once; 0 is off                                                      |
| `skip_dirs`               | Directories a file walk never enters                                                                                                                                                                                              |
| `sanitize_skip`           | Files `sanitize-output` leaves alone                                                                                                                                                                                              |
| `slop`                    | The AI-tell lexicon `kit slop` scans with: rules by tier, the density word list and threshold, which tiers block, files to skip                                                                                                   |
| `deny_flags`              | Flags no check may set (`--fix`, `--watch`, ...)                                                                                                                                                                                  |
| `tools`                   | The CLIs session start reports as available or missing                                                                                                                                                                            |
| `chat_bundle`             | The skills and rules the claude.ai chat zip carries                                                                                                                                                                               |
| `log_max_lines`           | Line cap for the kit's JSONL logs                                                                                                                                                                                                 |
| Skill maps                | `skill_file_map`, `dependency_skills`, `skill_triggers`, `global_skills`: which pattern skills a file or dependency calls for                                                                                                     |

`kit config` prints the merged result, each key marked with the files that set it; `kit config --check` flags unknown keys and wrong types.

## Presets

A preset is a set of these keys under a name. Put `preset: <name>` in your overlay, and its file merges between `kit.yml` and your overlay, so your own keys still win. Each file in [`presets/`](../presets) is a few lines:

| Preset     | Sets                                                                                                                       |
| ---------- | -------------------------------------------------------------------------------------------------------------------------- |
| `light`    | `step_pause: plan`, `disabled_rules: [plan-critic, skills-nudge]`: one stop per plan, no plan-critic pass, no skill nudges |
| `standard` | Nothing: `kit.yml` as it is                                                                                                |
| `strict`   | `guard_skills: true`, `sanitize_typography: true`, `claims.timeout: 240`: both opt-in guards and the claim checker on      |

`CLAUDE_GUARD_SKILLS=1` and `CLAUDE_SANITIZE_TYPOGRAPHY=1` in your settings `env` still turn those two guards on.

## Going back a version

Pin the marketplace to a release tag and reinstall:

```sh
claude plugin marketplace remove ku5ic
claude plugin marketplace add ku5ic/claude-kit#v0.12.0
claude plugin install kit@ku5ic
```

Removing the marketplace uninstalls the kit and drops its `/plugin` options, such as `subagent_rows`, so set them again. The kit stays on that tag until you add the marketplace back without `#<tag>`.
