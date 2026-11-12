# claude-kit

Working on the kit itself. Build, test, and release steps live in CONTRIBUTING.md; this file holds only what that doesn't.

## One source of truth

Every behavior the kit applies everywhere has exactly one owner, listed below. Before adding or changing guidance, find its owner here. Change it there, or nowhere. Every other file links to the owner or says nothing about the topic. It never restates, paraphrases, or reminds.

- Data and mechanics belong to code: `kit.yml` and the `kit` binary. Prose never re-describes what a subcommand does.
- `kit.yml` stays small enough for a person to read. Data another file already holds is derived from that file, and a new key needs the author's yes first.
- When two files disagree, the owner wins and the other file is fixed.
- A topic missing from this table gets a row before it gets text anywhere else.

| Topic                                                                                                                                                               | Owner                                                                                                         |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| Reply length, shape, where output goes, writing files with Edit/Write, code-block tags, external-communication tone, review comment shape                           | `skills/kit-rules/reference/output.md`                                                                        |
| Evidence, confidence labels, never invent, stop after three failed fixes                                                                                            | `skills/kit-rules/reference/evidence.md`                                                                      |
| Minimal change, precedent, blast radius, dead code, comments, abstractions, simplification criteria, less code                                                      | `skills/kit-rules/reference/change.md`                                                                        |
| Which pattern a size, spread, or repetition signal calls for                                                                                                        | `skills/engineering-fundamentals/reference/design-patterns.md`                                                |
| Size, nesting, branch, and coupling thresholds                                                                                                                      | `skills/engineering-fundamentals/reference/metric-thresholds.md`                                              |
| Done and verification                                                                                                                                               | `skills/kit-rules/reference/verify.md`                                                                        |
| Git and commit policy, side effects, shell mechanics, plans and the step pause, ask vs proceed, AskUserQuestion, review findings, which skills are procedure skills | `skills/kit-rules/reference/workflow.md`                                                                      |
| Subagent use and agent defaults                                                                                                                                     | `skills/kit-rules/reference/subagents.md`                                                                     |
| CLI choice, read budget, calling bin scripts, scratch and plan location                                                                                             | `skills/kit-rules/reference/tooling.md`                                                                       |
| Artifact naming and path                                                                                                                                            | `kit scratch-dir`                                                                                             |
| Resolving a linked ticket or doc before work starts                                                                                                                 | `skills/investigate` step 0                                                                                   |
| Report shape and severity                                                                                                                                           | `skills/report-format`                                                                                        |
| Test design                                                                                                                                                         | `skills/test-patterns`                                                                                        |
| A reference skill's shape: sources in `sources.md`, description and size budget                                                                                     | `go/e2e/kit_repo_test.go`; keeping `sources.md` current is `skills/meta/reference/refresh.md`                 |
| Red-green-refactor cycle                                                                                                                                            | `skills/engineering-fundamentals/reference/red-green-refactor.md`                                             |
| Guard lists, skill maps, deny lists and flags, gate env and timeout, `on_edit` formatting, the CLI tools list, the AI-tell lexicon and its tiers                    | `kit.yml`                                                                                                     |
| When a plan stops for review                                                                                                                                        | `kit.yml` `step_pause:`, its lines in `go/internal/hooks/plan.go`                                             |
| Presets: which keys each sets, and where they merge                                                                                                                 | `presets/*.yml`, merged by `go/internal/config`                                                               |
| Which checks, fixers, hooks, and package managers a project has                                                                                                     | its own configs, read by `go/internal/sources`; gap-fill (`go/internal/gapfill`) for what they leave open     |
| What runs at each checkpoint: full gate, Stop, per edit, commit message                                                                                             | `go/internal/enforce`                                                                                         |
| Languages a project uses, and its subprojects                                                                                                                       | the manifests `go/internal/sources` reads (`Languages`, `Anchors`)                                            |
| Where a tool's binary comes from, and when PATH is allowed                                                                                                          | `go/internal/resolve`                                                                                         |
| How task bodies and CI steps are read                                                                                                                               | `go/internal/classify`                                                                                        |
| Finding AI tells in text, and the `unslop: allow` marker                                                                                                            | `go/internal/slop`                                                                                            |
| What an AI tell is and how to rewrite one, in prose and in UI                                                                                                       | `skills/slop-patterns`                                                                                        |
| What edit weakens a check: a skipped or focused test, a silenced linter, lost assertions                                                                            | `go/internal/weaken`, its patterns in `kit.yml` `weakened_checks:`                                            |
| A package an add verb or a new import names that may not exist                                                                                                      | `go/internal/pkgcheck`, its registry lookups in `kit.yml` `package_lookups:`                                  |
| A design token a UI edit uses that the project never defines, and contrast ratios                                                                                   | `go/internal/design`                                                                                          |
| Telling an MCP tool that writes from one that only reads                                                                                                            | `go/internal/connector`, its verbs in `kit.yml` `connectors:`                                                 |
| Checking a plan's, subagent report's, scratch report's, or post's claims against the session's evidence and tools before they're presented                          | When: `go/internal/hooks/claims.go`. How: `agents/claim-checker.md`. The hook's own run: `go/internal/claims` |
| Which skill references get read before work: each skill's "Before \| Read" table                                                                                    | The table and its preamble; for `global_skills`, `requiredReferences` in `go/internal/stackctx`               |
| Whether a plan's or a gatherer report's `file:line` citations exist                                                                                                 | When: `go/internal/hooks/citations.go`. How: `go/internal/cite`                                               |
| A deliverable over its word budget for its kind                                                                                                                     | Budgets: `kit.yml` `size_budgets:`. The check: `go/internal/hooks/size.go`                                    |
| What a gatherer is asked, and when a plan step asks it                                                                                                              | `skills/investigate`, "Gatherer questions"                                                                    |
| Which skills and rules the claude.ai chat bundle carries                                                                                                            | `kit.yml` `chat_bundle:`                                                                                      |
| How the chat bundle is built from them                                                                                                                              | `go/internal/chatbundle`                                                                                      |
| Why each rule says what it says, for people (the model never loads it)                                                                                              | `docs/rules.md`                                                                                               |
| What the kit does and how it works, for people; the code wins on any disagreement                                                                                   | `README.md`, `docs/guards.md`, `docs/checks.md`, `docs/configuration.md` (ADR 0025)                           |
| Why an architecture decision was made, and what replaced it                                                                                                         | `docs/adr/`                                                                                                   |
| The sections of this repo's PR description                                                                                                                          | `.github/pull_request_template.md`                                                                            |
| Personal register and typography                                                                                                                                    | the user's own `~/.claude/rules/voice.md`, never the kit                                                      |

Skills and agents carry procedure and stack knowledge only. When one needs a rule above, it cites `skills/kit-rules/reference/<file>.md` instead of restating it.

## Run the checkout's kit

In this repo, call `bin/kit`, never bare `kit`: on PATH, `kit` is the installed release, which plans and checks with the old code. With `KIT_DEV=1`, `bin/kit` builds from `go/`. This overrides `skills/kit-rules/reference/tooling.md` section 2 here only.

## Rules are live

A session reads the rules from the plugin root it runs from, when it loads the kit-rules skill. A session running this checkout picks up a rule edit at its next load, so finish a rules change in one sitting.

## Hook changes

Before applying a review finding to a hook, say what the user sees when it fires. If the answer is more blocks, ask first.

## Architecture decisions

- Before changing something an ADR in `docs/adr/` decided, read it. A change that contradicts it needs a new ADR that supersedes it, in the same change.
- Create one with `VISUAL=true adr new <title>` (`-s <n>` to supersede, `-l "<n>:Amends:Amended by"` to amend), then fill it in with Edit, so the write hooks run.
- An accepted ADR changes only in its status line, which `adr` writes.
- Every sentence traces to a commit, a PR body, the README, or the code. Name the source in the Context.
- The `Date:` line is the day the decision landed on main: the merge of the PR it names, else the commit that made it. `adr new` writes today's date; replace it when the decision is older.
