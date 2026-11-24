<img src="assets/claude-kit-mark.svg" alt="" width="40">

# Dependencies

What the kit needs installed, what a fresh macOS or Linux install already has, and what each optional tool adds.

The kit is opinionated about tools. Its rules tell Claude to search with `rg` over grep, find files with `fd` over find, match code by shape with `ast-grep`, and read JSON with `jq` ([the tooling rule](../skills/kit-rules/reference/tooling.md)). None of these ship with macOS or Linux, except `jq` on macOS 15 and later and on Ubuntu. Without them the kit still runs and Claude falls back to what's there, with less to go on.

## Required

Without these the kit doesn't run. A fresh install doesn't always have them:

| Tool                            | Why                                                         | A fresh install                                                                                                                                                               |
| ------------------------------- | ----------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `git`                           | every guard and command                                     | macOS asks to install the Command Line Tools the first time `git` runs. Ubuntu Server and Fedora Workstation have it; Ubuntu Desktop, Debian, and every container image don't |
| bash                            | the launcher and the hooks                                  | macOS (3.2) and every Linux distro but Alpine, which needs `apk add bash`                                                                                                     |
| `curl`, `sha256sum` or `shasum` | downloading and checking the `kit` binary, once per version | macOS has both. Ubuntu Desktop and the Debian, Ubuntu, and Alpine images lack `curl`                                                                                          |

Without a downloaded binary, the hooks let everything through and session start says why.

The kit needs Claude Code 2.1.323 or later, set in `.claude-plugin/plugin.json` `engines`. CI tests against 2.1.326.

## Optional

None of these ship with macOS or Linux, except `jq` (macOS 15 and later, Ubuntu). Homebrew has each under its own name, except `rg` (`ripgrep`), `difft` (`difftastic`), `mmdc` (`mermaid-cli`), and `adr` (`adr-tools`). On Linux, `ast-grep`, `scc`, `mermaid-cli`, `qsv`, `actionlint`, and `trufflehog` aren't in the Debian, Ubuntu, or Fedora repos, and `difftastic` isn't in Debian's or Ubuntu's, so take them from Homebrew or each project's releases. Debian and Ubuntu install `fd` as `fdfind`, which the kit finds, and the `<tooling>` block tells Claude to call it by that name.

A missing one, except `glab` and `claude`, is named on the session-start `<tooling>` block's `missing:` line, and Claude leaves it unused. Install `ast-grep` and `jq` first, since their rows lose the most.

| Tool         | Used by                                                                                                                             | Without it                                                                                                   |
| ------------ | ----------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| `gh`         | `/pr-*`, `/deps`, `/code-history`, `/code-challenge`, `/code-migrate` across an org, `/write-tickets`, linked tickets               | the host's MCP connector; with neither, `/pr-*` and `/deps` stop and `/code-history` works from commits only |
| `glab`       | `/pr-*`, `/deps`, `/code-history`, `/code-challenge`, review checkout, on GitLab                                                    | the host's MCP connector, else `/code-history` works from commits only                                       |
| `rg`         | text search everywhere, `/audit-*`, `/code-*`, the gatherer agent                                                                   | search uses grep                                                                                             |
| `ast-grep`   | `kit code-graph imports` (`/code-visual`, `/code-impact`), `kit blast-radius`, `/code-migrate`, `/code-contract`, searches by shape | `kit code-graph imports` fails, `kit blast-radius` exits 2; `rg` stands in for shape search                  |
| `fd`         | file search, `/audit-*`, `/code-*`                                                                                                  | file search uses find                                                                                        |
| `tokei`      | `/audit-debt`, `/code-visual`                                                                                                       | file sizes come from reading files                                                                           |
| `scc`        | `/code-visual` hotspots                                                                                                             | the hotspot quadrant is skipped                                                                              |
| `mmdc`       | `/code-visual` diagram rendering                                                                                                    | diagrams go unrendered, and the report says so                                                               |
| `gitleaks`   | the commit guard's staged-diff scan, `/audit-security`                                                                              | the commit scan is skipped                                                                                   |
| `trufflehog` | `/audit-security` history scan                                                                                                      | history goes unscanned                                                                                       |
| `actionlint` | `/audit-ci`                                                                                                                         | `/audit-ci` skips its actionlint step                                                                        |
| `hyperfine`  | `/audit-perf` timings                                                                                                               | `/audit-perf` makes no timing claim                                                                          |
| `difft`      | `/code-history`, dropping reformat-only commits                                                                                     | reformat-only commits stay in the ranking                                                                    |
| `adr`        | `/write-adr` creating an ADR                                                                                                        | `/write-adr` numbers and names the file by the folder's pattern                                              |
| `jq`         | JSON questions, `/code-visual`, `/meta-retro`                                                                                       | Claude reads the JSON, but `/meta-retro` and `/code-visual` hotspots can't run                               |
| `gron`       | grepping flat JSON paths                                                                                                            | `jq` stands in                                                                                               |
| `yq`         | YAML and TOML questions                                                                                                             | Claude reads the file instead                                                                                |
| `qsv`        | CSV and TSV questions                                                                                                               | Claude reads the file instead                                                                                |
| `git-absorb` | fixup commits from staged hunks                                                                                                     | `git rebase -i` with fixups instead                                                                          |
| `claude`     | gap-fill sorting your CI and hook commands, and, with the strict preset, the claim check on plans, reports, and posts               | those commands are skipped and named, and the claims go unchecked                                            |
