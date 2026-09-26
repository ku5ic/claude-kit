# Tooling

Which CLI to reach for, how to call the bin scripts, and where temporary files go.

## 1. Reach for the CLI first

When a deterministic CLI can answer the question, call it before reading files and reasoning. Fewer tokens wins: one CLI call beats a targeted read beats a broad grep beats reasoning from memory. The `<tooling>` block's `available:` and `missing:` lines say which of these are on PATH; don't reach for a missing one.

| Question                           | Tool                               | Over                               |
| ---------------------------------- | ---------------------------------- | ---------------------------------- |
| Locate code by text                | `rg`                               | grep, find -name, Read walks       |
| Locate code by shape (AST)         | `sg`                               | rg, when structure is the question |
| Find files/paths                   | `fd`                               | find                               |
| Repo size, language breakdown      | `tokei`                            | reading files to estimate          |
| Secrets, quick                     | `gitleaks`                         | manual grep                        |
| Secrets, deep history              | `trufflehog git file://.`          | gitleaks                           |
| Timing a perf claim                | `hyperfine`                        | opinion                            |
| JSON                               | `jq`, or `gron` to grep flat paths | substring matching                 |
| YAML/TOML                          | `yq`                               | manual parsing                     |
| CSV/TSV                            | `qsv`                              | ad-hoc awk                         |
| In-place substitution              | `sd 'find' 'repl' file`            | sed -i                             |
| Buffer stdin for in-place pipes    | `cmd \| sponge file`               | temp-file dance                    |
| Fixup commits from staged hunks    | `git absorb`                       | rebase -i + fixup                  |
| Git outside cwd                    | `git -C <dir>`                     | cd <dir> && git (prompts)          |
| Validate a GitHub Actions workflow | `actionlint`                       | reading YAML by eye                |

Factual question (how big, what secrets, how fast, what is in this JSON): reach for the tool. Interpretive question (is this correct, does this design hold): reading and reasoning is correct.

### Budget

- Cap `git log` at `-20` unless a wider window is justified.
- Do not `cat` files over 500 lines without a reason. Use line ranges.
- Once located, read the matched section, not the whole file.
- Do not re-read a file in the same session unless an edit changed it.
- For diffs, prefer `git diff <base>..HEAD -- <path>` over unfiltered diff.
- Reference scratch artifacts by path; do not inline their contents.
- Skip for any glob, grep, or read: `node_modules/**`, `.next/**`, `dist/**`, `build/**`, `coverage/**`, `.turbo/**`, `.cache/**`, `vendor/**`, `target/**`, `out/**`, `storybook-static/**`, `.pnpm-store/**`, `__pycache__/**`, `.venv/**`, `venv/**`.

## 2. Call bin scripts by bare name

The kit's `bin/` is on PATH (the plugin's `bin/`, or `~/.claude/bin` in a symlinked install). Call every script there by bare name, never by path.

- Correct: `project-name.sh`, `run-checks.sh`, `git-base.sh main`, `scratch-dir.sh`
- Wrong: `$HOME/.claude/bin/run-checks.sh`, `./bin/run-checks.sh`, `bash run-checks.sh`

Arguments go as plain positional args after a space. Do not wrap a call in `bash` or `sh`; the shebang handles it. Inline skill injection uses the same form: `` !`project-name.sh` ``.

Permission allows for these scripts are written against the bare command, so a pathful or `bash`-wrapped call won't match one and triggers a permission prompt.

Script-to-script calls inside the bin scripts are exempt; they resolve paths internally.

## 3. Scratch

Scratch is whatever `scratch-dir.sh` prints: `<project-root>/.claude/scratch/` inside a recognized project (a git worktree, or a stack sentinel matched during the ancestor walk), `$HOME/.claude/scratch/` everywhere else.

Three sibling directories under a project's `.claude/`, each with one job:

| Directory          | Holds                                                                            | Tracked?        |
| ------------------ | -------------------------------------------------------------------------------- | --------------- |
| `.claude/scratch/` | Throwaway work: POCs, one-off scripts, logs, downloads, screenshots              | No - gitignored |
| `.claude/plans/`   | Plan-mode files, via `plansDirectory: ".claude/plans"`                           | No - gitignored |
| `.claude/tasks/`   | Handover plans a person is meant to read - written by hand, no skill writes here | Yes             |

**Everything temporary goes to scratch**: reports, previews, test artifacts, proof-of-concept scripts, one-off debug files, downloads, screenshots, logs. Plans are the exception - they have their own directory above. This overrides two competing defaults: the harness's per-session `/tmp` scratchpad (the project tier survives the session), and ad hoc paths under `~/.claude/`. The harness's own memory stores are the one carve-out.

### Never the project root

Resolve the destination first, then pass it explicitly:

- `curl -o "$(scratch-dir.sh)/<name>"`, not `curl -O`
- a screenshot or export tool's `out_dir` argument, not its default
- `cmd > "$(scratch-dir.sh)/<name>.log"`, not `cmd > out.log`

Never default to `.`, a bare filename, or whatever directory the tool picks. A stray file in the project root pollutes `git status`, risks being committed, and lands in every clone.

- Read web pages and docs with WebFetch, never `curl` to disk. If you only need to search a page, pipe it (`curl ... | rg`) instead of saving it.
- After any download, and after any subagent that has Bash returns, run `git status --short`. A new untracked file you didn't intend to create gets moved to scratch or flagged before you do anything else.
- A subagent prompt that may write files names `$(scratch-dir.sh)` as the only place it may write.

**Hard rule: `curl` and `wget` download only into scratch.** `guard-bash.sh` blocks any output file, output directory, or `>` redirect that isn't stdout, `/dev/null`, `"$(scratch-dir.sh)/..."`, or a path inside a `.claude/scratch` directory, plus `curl -O`/`-J` without a scratch `--output-dir` and `wget` with no output flag. A shell alias expands after the hook runs, so an alias that adds `-O` would slip past it; keep such aliases out of Claude Code sessions (`CLAUDECODE=1`). Tool-driven writes (a browser screenshot's `out_dir`, an MCP server's download path) are invisible to the hook and rely on this rule.

### Naming

Structured artifacts (reports, reviews, audits) go to the path `scratch-dir.sh <kind> [slug]` prints: `<kind>-<slug>-<YYYYMMDD-HHMM>.md` in the scratch directory.

Test artifacts and POC files need no fixed shape - name them sensibly, but keep them under the resolved directory.

Plans are the exception to the timestamp. `.claude/plans/` is browsed by eye and shares a directory with the harness's own plan-mode files, so a plan is `plan-<task-slug>.md` - no date, slug capped at four words. Its age comes from the file's birth time (`stat -f %B`), which beats a filename date anyway: editing a plan no longer hides how old it is.

Reading the most recent artifact of a kind, always filtered to the resolved directory:

```sh
ls -t "$(scratch-dir.sh)"/<kind>-*.md | head -1
```

Never read across projects. If none exists for this project, run the predecessor command first.

Retention, the prune registry, and `.claude/` write gating are documented in the headers of `scratch-dir.sh`, `scratch-rotate.sh`, and `plans-dir.sh`.
