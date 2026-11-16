# Tooling

Which CLI to reach for, how to call the bin scripts, where temporary files go, and how to find the code host and connectors.

## 1. Reach for the CLI first

- A factual question (how big, what secrets, how fast, what's in this JSON) goes to a CLI before any file read. An interpretive one (is this correct, does this design hold) gets read and reasoned about.
- A tool the `<tooling>` block's `missing:` line names stays unused. A step that needs it is skipped and named, with the tool, in the report or reply, and the rest of the task runs.

| Question                           | Tool                               | Over                                                     |
| ---------------------------------- | ---------------------------------- | -------------------------------------------------------- |
| Locate code by text                | `rg`                               | grep, find -name, Read walks                             |
| Locate code by shape (AST)         | `ast-grep`                         | rg, when structure is the question                       |
| Find files/paths                   | `fd`                               | find                                                     |
| Repo size, language breakdown      | `tokei`                            | reading files to estimate                                |
| Complexity per file                | `scc`                              | reading files to judge                                   |
| Secrets, quick                     | `gitleaks`                         | manual grep                                              |
| Secrets, deep history              | `trufflehog git file://.`          | gitleaks                                                 |
| Timing a perf claim                | `hyperfine`                        | opinion                                                  |
| JSON                               | `jq`, or `gron` to grep flat paths | substring matching                                       |
| YAML/TOML                          | `yq`                               | manual parsing                                           |
| CSV/TSV                            | `qsv`                              | ad-hoc awk                                               |
| Who imports a file                 | `kit blast-radius <file> [symbol]` | `rg`, without `ast-grep` or for a language it can't scan |
| Fixup commits from staged hunks    | `git absorb`                       | rebase -i + fixup                                        |
| Git outside cwd                    | `git -C <dir>`                     | cd <dir> && git                                          |
| Validate a GitHub Actions workflow | `actionlint`                       | reading YAML by eye                                      |
| Check a Mermaid diagram renders    | `mmdc`                             | reading the fence by eye                                 |
| A PR, issue, or API call           | the host's CLI (section 4)         | fetching the host's pages                                |
| Diff that ignores reformatting     | `difft`                            | `git diff -w`                                            |
| Read or create an ADR              | `adr`                              | listing `docs/adr` by hand                               |

### Budget

- A reference a loaded skill's "Before | Read" table links is outside this budget. Read it whole, because the skill's advice is written on top of it.
- Cap `git log` at `-20` unless a wider window is justified.
- A file over 500 lines gets read by line range.
- Once located, read the matched section, not the whole file.
- A file already read this session gets re-read only after an edit changed it.
- A diff gets a path filter, `git diff <base>..HEAD -- <path>`.
- A scratch artifact gets referenced by path, never inlined.
- Skip for any glob, grep, or read: the dirs the `<repo-context>` block's `skip-dirs:` line names.

## 2. Call kit by bare name

- Call the plugin's tools as `kit <subcommand>` (`kit scratch-dir`, `kit git-base`, `kit blast-radius`). Never by path or through `bash`.

## 3. Scratch

Scratch is whatever `kit scratch-dir` prints, the `scratch:` line of `<repo-context>`.

| Directory          | Holds                                                               | Tracked?       |
| ------------------ | ------------------------------------------------------------------- | -------------- |
| `.claude/scratch/` | Throwaway work: POCs, one-off scripts, logs, downloads, screenshots | No, gitignored |
| `.claude/plans/`   | Plan files, when the `plansDirectory` setting points here           | No, gitignored |
| `.claude/tasks/`   | Handover plans a person is meant to read, written by hand           | Yes            |

- A temporary file (a report, preview, test artifact, POC, debug file, download, screenshot, log) goes to scratch, over the harness's `/tmp` scratchpad and any ad hoc path under `~/.claude/`.
- A command that writes a file gets the scratch path passed explicitly (`curl -o "$(kit scratch-dir)/<name>"`, a tool's `out_dir`, `> "$(kit scratch-dir)/<name>.log"`).
- A web page or doc gets read with WebFetch, or searched with `curl ... | rg`. Never saved to disk to read.
- After a download, or after a subagent with Bash returns, run `git status --short`. A file you didn't mean to create gets moved to scratch or flagged first.
- A subagent prompt that may write files names `$(kit scratch-dir)` as its only place to write.
- In plan mode, where Claude Code lets an agent write only the plan file, a report goes where the harness allows, and the reply names that path.
- A structured artifact (a report, review, or audit) goes to the path `kit scratch-dir <kind> <slug>` prints. A plan is `plan-<task-slug>.md` in the folder `plansDirectory` points to, else Claude Code's default `~/.claude/plans/`, slug capped at four words.

## 4. Code host and connectors

- The code host is the host part of `git remote get-url origin`.
  - `github.com`, or a host `gh auth status` lists: `gh`.
  - `gitlab.com`, or a host `glab auth status` lists, with `glab` on PATH: `glab`.
  - Anything else, or the CLI missing: the host's MCP connector, found as below. None, say so and work from git alone.
- A tracker, chat, monitoring, docs, or design tool is found by ToolSearch on its kind and verb (`issue create`, `send message`, `query logs`). One match: use it. Several: ask which. None: say so.
- A URL names its service by its domain. Resolve it the same way.
