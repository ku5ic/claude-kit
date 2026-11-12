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
- Skipping commit hooks through `core.hooksPath` or `HUSKY=0`, or a git alias that hides `push --force`.
- Credential reads through `env`, `printenv`, a search over `~/.aws`, a key path held in a variable, `security find-generic-password`, or `gh auth token`.
- `claude plugin uninstall kit`.
- A reply with no tool call, which no hook sees.
- The release workflow's actions on version tags, not commit pins.
