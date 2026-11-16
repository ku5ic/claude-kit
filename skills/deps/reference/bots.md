# Dependency bots

Which bot a repo runs, how to tell its PRs apart, and how to ask it to rebase.

| Bot        | Config file                                                                                                                                                                                   | Branch prefix                                                                     | Rebase request                                         |
| ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- | ------------------------------------------------------ |
| Dependabot | `.github/dependabot.yml`                                                                                                                                                                      | its `pull-request-branch-name` `prefix` and `separator`, by default `dependabot/` | the comment `@dependabot rebase`                       |
| Renovate   | `renovate.json`, `.jsonc`, or `.json5`, the same under `.github/` or `.gitlab/`, `.renovaterc`, `.renovaterc.json`, `.jsonc`, or `.json5`, or a `renovate` key in `package.json` (deprecated) | its `branchPrefix`, by default `renovate/`                                        | the label its `rebaseLabel` names, by default `rebase` |

- Renovate takes the first config file it finds, in that order.
- A commit pushed to a Renovate branch stops Renovate updating it, until the PR is merged or renamed and closed. A rebase label still regenerates the branch.
- Dependabot stops rebasing a PR left unmerged for 30 days. `@dependabot recreate` rebuilds it, overwriting edits.
