# Security

Report a vulnerability privately through [GitHub's private reporting](https://github.com/ku5ic/claude-kit/security/advisories/new), not a public issue. A guard bypass counts: a command, path, or commit message that should be blocked and isn't.

Only the latest release is supported.

## Known gaps

The guards read a command line, not what it does when it runs. A pass means the kit says nothing; Claude Code's own permission prompt still applies unless the command is allow-listed or the session runs in a bypass mode. On 2026-11-11, 199 probe commands, most of them destructive, credential-reading, or guard-disabling, gave 105 blocks, 29 asks, and 65 passes. These routes pass, and reports of them aren't new bypasses:

- Inline code in an interpreter: `python3 -c`, `perl -e`, `ruby -e`, `node -e`, `php -r`, `awk` with `system()`, or a heredoc fed to one.
- Indirection: a command or flag held in a variable, an alias or function, `source <(...)`, `bash <(...)`, `trap`, or a script written to disk and then run.
- Wrappers and remote runners: `busybox`, `script`, `parallel`, `find -exec sh -c`, `xargs sh -c`, `ssh`, `docker run`, `osascript`.
- Opaque task runners: `go run`, `make`, `npm run`.
- Other ways to lose work: `git rm -rf .`, `rsync --delete`, `mv` of the home directory, truncating a file, `git stash drop`, `git update-ref -d`, `gh api -X DELETE` on a branch, `gh pr merge --admin`.
- Skipping commit hooks through `git config core.hooksPath`, a config file passed in `GIT_CONFIG_GLOBAL` or `GIT_CONFIG_SYSTEM`, or `HUSKY=0`, or a git alias that hides `push --force`. A `git -c core.hooksPath=...`, or the same through `GIT_CONFIG_KEY_<n>` or `GIT_CONFIG_PARAMETERS`, blocks.
- Credential reads through `env`, `printenv`, a search over `~/.aws`, a key path held in a variable, a glob in a `<` redirect, `security find-generic-password`, or `gh auth token`.
- Staging a credential file the repo doesn't ignore with `git add .` or `git add -A`. `git add` of the file by name asks.
- `claude plugin uninstall kit`.
- A reply with no tool call, which no hook sees.
- A recursive search with no glob naming a credential file, over a directory that holds a loose `*.pem`, outside the project or through `grep -r` or a chained `rg`. Catching it means walking the tree.
- A Bash command's writes are checked after they land, and only in files `git add -A` sees, so an ignored test file can take a `t.Skip()`.
- A file a background agent changes after Claude's turn ends counts as yours: the next turn starts from it, and no check runs on it.
- One `kit:plan-critic` run covers every later version of the plan it read.
- The step pause is an instruction. No hook stops work that runs past it.
- After a `/code-review`, a Bash command that writes code isn't held until you pick which findings to fix. Edits through Claude's edit tools are.
- The release workflow's actions on version tags, not commit pins.
