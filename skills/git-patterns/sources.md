# git-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- https://www.conventionalcommits.org/en/v1.0.0/
- https://git-scm.com/docs/git-rebase
- https://git-scm.com/docs/git-reflog
- https://git-scm.com/docs/git-bisect
- https://git-scm.com/docs/git-worktree
- https://git-scm.com/docs/git-push
- https://git-scm.com/docs/git-switch

### Version notes

Checked: 2026-11-10 against https://git-scm.com, https://github.com/git/git/tags, https://git-scm.com/docs/git-rebase, https://git-scm.com/docs/git-bisect and https://www.conventionalcommits.org/en/v1.0.0/

- git 2.56.0 (2026-09-28), 2.56.1 tagged: no guidance change. Corrected the merge-commit squash warning per git-rebase (merges are dropped by default) and added bisect's exit 125.
- `--force-if-includes` (2.30+): pairs with `--force-with-lease` against the background-fetch gap; added to the force-push guidance 2026-09-12.
- Conventional Commits spec is 1.0.0; unchanged.
