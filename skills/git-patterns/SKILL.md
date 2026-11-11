---
name: git-patterns
description: Git patterns for Conventional Commits, rebase, reflog recovery, bisect, worktrees, and stash. Use in a git repository or for any git operation.
---

# Git Patterns

Review checklist and the handful of git behaviors that are easy to get wrong. Command syntax is in the man pages; this file is what to flag and what to reach for.

## Conventional Commits

```text
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

- `feat` bumps MINOR, `fix` bumps PATCH, everything else (`docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`) bumps nothing.
- Breaking change: `!` after the type/scope, or a `BREAKING CHANGE:` footer. Either maps to MAJOR.
- Match the project's existing style before assuming Conventional Commits (`skills/kit-rules/reference/workflow.md`).

---

## Reach for these

| Situation                                | Tool                                                                                      |
| ---------------------------------------- | ----------------------------------------------------------------------------------------- |
| Commits lost after a bad rebase or reset | `git reflog`: every HEAD move is recorded; branch off `HEAD@{n}`                          |
| Need each commit in a series to build    | `exec` lines in the `rebase -i` todo (`exec make test`)                                   |
| Hunting the commit that broke something  | `git bisect run <cmd>`: automates the whole search; exit 0 = good, 125 = skip             |
| Reviewing a branch mid-edit on another   | `git worktree add <dir> <branch>`: separate working tree, shared `.git`                   |
| Fixups for already-staged hunks          | `git absorb` (see `skills/kit-rules/reference/tooling.md`) beats hand-written `rebase -i` |
| A file is ignored and you cannot see why | `git check-ignore -v <path>` names the rule and its source line                           |

---

## Anti-patterns

**failure: force-pushing to a shared branch**
The rule is `skills/kit-rules/reference/workflow.md` section 1. To force-push your own remote branch, use `--force-with-lease --force-if-includes`: the first fails if someone else pushed since your last fetch, the second also fails when a background fetch updated the ref without you seeing it.

**failure: committing secrets**
`git rm` does not remove it from history; the secret is still in every clone. Remediation is rotation: security-patterns' `reference/secrets.md`.

**warning: merge commit spam on feature branches**
Merging `main` into a feature branch repeatedly to stay up to date produces a messy history. Prefer `git rebase main` on the feature branch to keep a linear history before merge.

**warning: squashing a merge commit into a feature commit**
A rebase drops merge commits from the todo list by default, and `--rebase-merges` recreates them rather than squashing them. Rebase onto `main` first to linearize history, then squash.

**warning: committing with `--no-verify`**
Skipping pre-commit hooks bypasses linting, type checks, and secret scanning. Only use if you understand exactly which hook is blocking you and why it is safe to skip in that moment.

**warning: `git reset --hard` as a routine undo**
It discards working tree changes with no reflog entry for them. `git reset --soft HEAD~1` keeps the changes staged and is the right default.

**info: no `.gitignore` for secrets patterns**
An `.env` or `*.key` file not listed in `.gitignore` will eventually be committed by accident. Add secret file patterns before any other work in a new repo.

---
