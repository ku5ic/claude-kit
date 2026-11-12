---
name: git-patterns
description: Git patterns for Conventional Commits, rebase, reflog recovery, bisect, worktrees, and stash. Load before any commit, rebase, or other git operation, however small.
---

# Git Patterns

Review checklist and the git behaviors that are easy to get wrong. Command syntax is in the man pages.

## Conventional Commits

- Match the project's existing style before assuming Conventional Commits (`skills/kit-rules/reference/workflow.md`).
- Message shape: `<type>[optional scope]: <description>`, then an optional body and optional footers, each after a blank line.
- `feat` bumps MINOR, `fix` bumps PATCH, and every other type (`docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`) bumps nothing.
- A breaking change: put `!` after the type or scope, or add a `BREAKING CHANGE:` footer. Either one bumps MAJOR.

## Reach for these

| Situation                                | Tool                                                                                   |
| ---------------------------------------- | -------------------------------------------------------------------------------------- |
| Commits lost after a bad rebase or reset | `git reflog`, then branch off `HEAD@{n}`                                               |
| Each commit in a series must build       | `exec` lines in the `rebase -i` todo, such as `exec make test`                         |
| Hunting the commit that broke something  | `git bisect run <cmd>`; exit 0 means good, 125 means skip                              |
| Reviewing a branch mid-edit on another   | `git worktree add <dir> <branch>`                                                      |
| Fixups for already-staged hunks          | `git absorb` (`skills/kit-rules/reference/tooling.md`), not a hand-written `rebase -i` |
| A file is ignored and you can't see why  | `git check-ignore -v <path>`, which names the rule and its source line                 |
| Force-pushing your own remote branch     | `git push --force-with-lease --force-if-includes`                                      |

## Anti-patterns

- Force-pushing to a shared branch: `failure`. Follow `skills/kit-rules/reference/workflow.md` section 1.
- A committed secret: `failure`. Rotate it, per security-patterns' `reference/secrets.md`; `git rm` leaves it in history.
- Merging `main` into a feature branch again and again to stay current: `warning`. Run `git rebase main` on the feature branch.
- Squashing a merge commit into a feature commit: `warning`. Rebase onto `main` first, without `--rebase-merges`, then squash.
- Committing with `--no-verify`: `warning`. Use it only when you know which hook blocks you and why skipping it is safe this once.
- `git reset --hard` as a routine undo: `warning`. Use `git reset --soft HEAD~1`, which keeps the changes staged.
- A new repo with no `.gitignore` patterns for secret files: `info`. Add patterns such as `.env` and `*.key` before any other work.
