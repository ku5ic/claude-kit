# 12. Release 0.12: one skill per procedure, each detecting its stack, and every scanned language covered

Date: 2026-11-16

## Status

Accepted

Supercedes [11. Release 0.12: one skill per procedure, each detecting its stack and citing its practice](0011-release-0-12-one-skill-per-procedure-each-detecting-its-stack-and-citing-its-practice.md)

## Context

Sources are the commits on `feat/kit-0-12-0`, each named where it's used. [11](0011-release-0-12-one-skill-per-procedure-each-detecting-its-stack-and-citing-its-practice.md) recorded the release up to `fa8ea9b`. Work went on after it, and a pre-1.0 release gets one ADR for all its behavior changes (`CLAUDE.md`, Architecture decisions), so this record carries 11's decisions and the rest.

- `/audit`, `/code`, `/write`, `/pr`, and `/meta` dispatched on a typed kind word, which the / menu can't complete (`eab6345`).
- Procedure skills named hosts and stacks, and the PR and attribution guards read `gh` text only (`8e8d543`).
- `/code-tour` answered a missing architecture model by asking the user to run `/code-visual` first (`6d2a480`).
- Procedure skills ran on practice nothing cited (`a2bf204`). `/write-commit` capped the subject at 72 characters where git's own figure is 50 (`d3fcfa9`).
- The claims gate matched only explore and debug reports, since 0.9.0, so an audit or impact report reached the user unchecked (`d22c22c`).
- `curl -d @.env`, `cat .e*`, and `git add .env` passed the bash guard (`e442220`).
- An overlay could add to a default list but never take from it, and `.env.example` couldn't be read (`7688fe5`).
- A doubtful guard rule could only be on or off (`c714806`). With the evals gone ([10](0010-release-0-11-no-release-eval.md)), nothing measured the claim checker's flags or how often each rule fired (`f20bc36`).
- `/meta-retro` covered incidents, sprints, and features, none of it the kit's own sessions (`871da10`).
- `kit blast-radius` read only JS, TS, and Python, so the blast-radius rule and `/pr-description`'s risk map were blind in Go, Rust, Java, and the rest (`5d7c66f`).
- There was no written way to add a stack the kit doesn't know, and a language touches several packages with nothing listing them (`820626e`, `5964a1e`).
- Java, Elixir, PHP, Dart, and Lua test files loaded no test-patterns, and blast-radius counted them as source (`bc306ad`).
- Debian and Ubuntu install `fd` as `fdfind`, so the kit listed `fd` as missing (`3f86a01`). Most of the kit's tools aren't preinstalled, and no skill said what to do when one is missing (`fda120e`).
- Scratch past 30 days was pruned only when someone ran `kit scratch-rotate`, and the kit's logs, caches, and home scratch outlived an uninstall (`6b0ab0d`).

## Decision

- Each procedure kind is its own user-only skill, `/audit-perf`, `/pr-review`, `/write-commit`, and so on. The dispatchers are gone (`eab6345`).
- A procedure skill assumes no language, framework, host, or vendor. It detects the stack, the code host (`gh` on GitHub, `glab` on GitLab, else the host's MCP connector), and the connectors, and takes stack specifics from the matching pattern skill. The rule is in `skills/kit-rules/reference/workflow.md` (`8e8d543`).
- A procedure skill runs the procedures it depends on itself. Input the user has to vet, a draft to post or review findings, still stops and names the skill (`6d2a480`).
- A procedure skill keeps the practice it runs in its own `reference/`, with primary sources in `sources.md` for `/meta-refresh`. The e2e check holds every user-only skill to a `sources.md`, or to its `unversioned` list with the reason. The rule is in `skills/meta-skill/SKILL.md`, the check in `go/e2e/kit_repo_test.go` (`a2bf204`, `d3fcfa9`).
- The claims gate checks every review, audit, and code report, and the session retro's (`d22c22c`, `7ec6f07`). Its kinds are in `go/internal/hooks/claims.go`, and a test fails when an audit or code skill's report kind is missing from them (`768f234`).
- `curl`'s `@` operands count as reads, an unquoted glob naming a credential file blocks, `git add` of one asks, and a snapshot rewrite with no other code change asks (`e442220`).
- An overlay's `"!x"` removes a default list item, `sensitive_paths` takes `!` exceptions (`7688fe5`), and a rule in `shadow_rules` logs its would-be blocks and lets the call through (`c714806`).
- Each claim-checker flag goes to `claims.jsonl` as kept or dropped, and `kit stats` reads it with `guards.jsonl` (`f20bc36`).
- A Read or edit of a file an accepted ADR cites gets that ADR's title and Decision once per session (`10f17ff`).
- `/meta-retro` is a retro on Claude Code sessions, one or many, every finding cited to the transcript (`7ec6f07`). The incident, sprint, and feature retro is `/write-retro` (`871da10`).
- The README keeps what decides an install and links the rest. `docs/commands.md`, `docs/skills.md`, `docs/cli.md`, and `docs/chat.md` join the people-facing pages [9](0009-release-0-10-the-engineer-in-the-loop-the-rules-as-a-gated-skill-a-visible-claim-checker.md) named, on its terms, and `go/e2e/kit_repo_test.go` fails when a skill has no row in the commands or skills page (`37dac6a`). `docs/dependencies.md` and `docs/adding-a-stack.md` join them too (`5d7c66f`, `820626e`).
- `codegraph` owns who uses a file, in every language, and `kit blast-radius` covers every language `kit code-graph` scans (`5d7c66f`).
- `/add-language` adds a language to the kit, in this repo only since it edits the kit's Go source. An e2e test fails when an extension `codegraph` scans has no comment syntax in `slop` or doesn't load engineering-fundamentals (`5964a1e`).
- Java, Elixir, PHP, Dart, and Lua test files are named by each runner's documented default, in the test-patterns globs and `weakened_checks.test_files` (`bc306ad`). The weaken guard's patterns cover JUnit and PHPUnit assertions and skips (`744b604`).
- An `.astro` file's new imports are checked for undeclared packages like a `.vue` file's (`a0dbd20`).
- The kit tries a tool's other names (`fdfind` for `fd`, `sg` for `ast-grep`), and the `<tooling>` block names the one to call (`3f86a01`). A step that needs a missing tool is skipped and named, and the rest of the task runs. The rule is in `skills/kit-rules/reference/tooling.md` (`fda120e`).
- Session start prunes scratch past 30 days, at most once a day, and `disabled_rules: [scratch-rotate]` turns it off (`6b0ab0d`, `186177d`). `kit clean` lists the kit's own state and `--yes` removes it, each path by name. Project scratch folders are listed for the user to delete (`6b0ab0d`, `e49a0ed`).

## Consequences

- `/audit <kind>` and the other dispatcher forms no longer work; users type `/audit-<kind>` (`eab6345`).
- A new procedure skill needs its sources researched, or a reason it has none, before the e2e check passes (`a2bf204`).
- The new skills of `768f234`, `a633296`, `871da10`, and `7ec6f07` weren't run end to end before release, and neither was anything on GitLab.
- `git add` of a credential file and a blind snapshot rewrite now stop for a question (`e442220`).
- `kit stats` gives the numbers a rule change is judged on, where the evals used to (`f20bc36`).
- JS, TS, and Python repos now need ast-grep for blast-radius, and say so (`5d7c66f`).
- A language added by hand that misses `slop` or the skill map fails the e2e test (`5964a1e`).
- Kotlin, Swift, C#, Scala, C, and C++ test files aren't named, since no doc there sets a default file name (`bc306ad`).
- In Java and PHP tests, a deleted assertion now asks and an added skip blocks (`744b604`).
- Old scratch in every registered project is deleted with no command run, unless `scratch-rotate` is disabled (`6b0ab0d`, `186177d`).
