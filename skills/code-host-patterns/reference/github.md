# GitHub

The `gh` CLI. `<repo>` is `owner/repo`; every `gh pr` call passes `-R <repo>`.

## Read

| Operation                       | Command                                                                                                                                                                                                  |
| ------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Repository path                 | `gh repo view --json nameWithOwner -q .nameWithOwner`                                                                                                                                                    |
| Current user                    | `gh api user -q .login`                                                                                                                                                                                  |
| The current branch's open PR    | `gh pr view --json number,url`                                                                                                                                                                           |
| Head SHA, title, body, files    | `gh pr view <n> --json headRefOid,headRefName,title,body,files`                                                                                                                                          |
| Diff                            | `gh pr diff <n>`                                                                                                                                                                                         |
| Head ref to fetch               | `+pull/<n>/head:refs/pr/<n>`                                                                                                                                                                             |
| Review threads                  | `gh api repos/<repo>/pulls/<n>/comments --paginate`, and `.../reviews` for review bodies                                                                                                                 |
| Review comments across the repo | `gh api "repos/<repo>/pulls/comments?since=<ISO date>" --paginate`, each with `body`, `path`, `html_url`, and `user.login`                                                                               |
| Link to a line at the head      | `<repo web URL>/pull/<n>/files#diff-<sha256 of path>R<line>`                                                                                                                                             |
| Code search across an org       | `gh search code '<term>' --owner <org> --json repository,path,url --limit 1000`, narrowed with `--language`, `--filename`, or `--extension`                                                              |
| Issue template                  | `.github/ISSUE_TEMPLATE/`                                                                                                                                                                                |
| PR template                     | `.github/pull_request_template.md`, `.github/PULL_REQUEST_TEMPLATE.md`, the alphabetically first `*.md` in `.github/PULL_REQUEST_TEMPLATE/`, `docs/pull_request_template.md`, `pull_request_template.md` |

A diff `gh pr diff` refuses as too large (it stops at 300 files) comes from the fetched head instead: `git diff <remote>/<base branch>...refs/pr/<n>`, after `git fetch <remote> <base branch>`. The line link's hash is `printf '%s' "<path>" | shasum -a 256`.

Code search covers the default branch only, files under 384 KB, and up to 1,000 results a query, at 10 requests a minute. Split a query by `--repo` or `--language` when it nears the cap.

## Post

| Operation                     | Command                                                                                                                                                       |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Review, line                  | `gh api -X POST repos/<repo>/pulls/<n>/comments -f commit_id=<sha> -f path=<file> -F line=<line> -f side=RIGHT -F body=@<body-file>`                          |
| Review, file                  | `gh api -X POST repos/<repo>/pulls/<n>/comments -f commit_id=<sha> -f path=<file> -f subject_type=file -F body=@<body-file>`                                  |
| Summary, or reply to a review | `gh api -X POST repos/<repo>/issues/<n>/comments -F body=@<body-file>`                                                                                        |
| Reply in a thread             | `gh api -X POST repos/<repo>/pulls/<n>/comments/<thread id>/replies -F body=@<body-file>`, the thread's top-level comment id                                  |
| Description, PR open          | `gh pr edit <n> --title "$(cat <title-file>)" --body-file <body-file>`                                                                                        |
| Issue, create                 | `gh issue create --title "$(cat <title-file>)" --body-file <body-file> [--label <name>] [--milestone <name>] [--parent <number>]`, which prints the issue URL |
| Description, no PR            | `gh pr create --head <branch> --title "$(cat <title-file>)" --body-file <body-file>`                                                                          |
| What the post log records     | the response's `html_url` and `created_at`                                                                                                                    |

The batched `POST /pulls/<n>/reviews` posts a whole review in one call. Content creation has secondary rate limits of 80 per minute and 500 per hour.

## Dependencies

| Operation                | Command                                                                                                                                             |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| Open bot PRs             | `gh pr list -R <repo> --state open --limit 200 --json number,title,body,headRefName,headRefOid,mergeable,mergeStateStatus,labels`, by branch prefix |
| Mergeable                | `mergeable` is `MERGEABLE` and `mergeStateStatus` is `CLEAN`; conflicted is `DIRTY` or `CONFLICTING`                                                |
| Security alerts          | Dependabot alerts, below                                                                                                                            |
| An alert's fixed version | `security_vulnerability.first_patched_version.identifier`, null when no release fixes it                                                            |
| Comment on a PR          | `gh pr comment <n> -R <repo> --body <text>`                                                                                                         |
| Add a label              | `gh pr edit <n> -R <repo> --add-label <label>`                                                                                                      |
| Wait for CI              | `gh pr checks <n> -R <repo> --watch`                                                                                                                |
| Merge                    | `gh pr merge <n> -R <repo> --squash --delete-branch --match-head-commit <sha>`                                                                      |

### Dependabot alerts

Fetch them with one bare GET and filter `state` in jq. `-f state=open` sends a body field, which makes gh POST, and this read-only endpoint answers a POST with 404. A bare `?state=open` trips zsh (`skills/bash-patterns/reference/quoting.md`). `--paginate` follows the Link header; `page`, `first`, and `last` were removed for these endpoints on 2025-10-14.

```bash
gh api "repos/<repo>/dependabot/alerts" --paginate \
  --jq '.[] | select(.state=="open") | {number, severity: .security_vulnerability.severity, cvss: .security_advisory.cvss.score, ghsa: .security_advisory.ghsa_id, pkg: .dependency.package.name, ecosystem: .dependency.package.ecosystem, scope: .dependency.scope, relationship: .dependency.relationship, manifest: .dependency.manifest_path, fixed: .security_vulnerability.first_patched_version.identifier, range: .security_vulnerability.vulnerable_version_range}'
```

The field paths were checked against a live payload on 2026-07-02. `state` is `open`, `fixed`, `dismissed`, or `auto_dismissed`. Read the status from `--include`'s `HTTP/2 <code>` line:

- 404 on the bare GET: alerts are off for the repo or hidden from the token. Say "unavailable", never "disabled". A 404 only with a filter attached means a malformed request.
- 403: the token lacks scope. A classic PAT needs `repo` or `security_events`; `gh auth refresh -s security_events` adds it.
