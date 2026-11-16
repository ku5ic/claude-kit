# GitLab

The `glab` CLI. `<repo>` is `group/project`, URL-encoded (`/` as `%2F`) in an API path; every `glab mr` call passes `-R <repo>`.

## Read

| Operation                       | Command                                                                                                                                                                                       |
| ------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Repository path                 | `glab repo view -F json`, its `path_with_namespace`                                                                                                                                           |
| Current user                    | `glab api user`, its `username`                                                                                                                                                               |
| The current branch's open PR    | `glab mr view -F json`, its `iid` and `web_url`                                                                                                                                               |
| Head SHA, title, body, files    | `glab mr view <n> -F json`: `sha`, `source_branch`, `title`, `description`, `diff_refs`                                                                                                       |
| Diff                            | `glab mr diff <n>`                                                                                                                                                                            |
| Head ref to fetch               | `+merge-requests/<n>/head:refs/pr/<n>`                                                                                                                                                        |
| Review threads                  | `glab api projects/<repo>/merge_requests/<n>/discussions --paginate`; a diff note's `position` holds its path and line                                                                        |
| Review comments across the repo | `glab mr list -R <repo> --merged --per-page 100 -F json`, then each MR's `glab api "projects/<repo>/merge_requests/<n>/notes?per_page=100" --paginate`, dropping notes whose `system` is true |
| Link to a line at the head      | `<project web_url>/-/blob/<sha>/<path>#L<line>`                                                                                                                                               |
| Code search across a group      | `glab api "groups/<id>/search?scope=blobs&search=<term>" --paginate`, each blob with `path`, `ref`, `startline`, and `project_id`                                                             |
| Issue template                  | `.gitlab/issue_templates/Default.md`, any letter case                                                                                                                                         |
| PR template                     | `.gitlab/merge_request_templates/Default.md`, any letter case                                                                                                                                 |

Group code search works only where advanced search is enabled. Without it, list the group's projects with `glab api groups/<id>/projects --paginate` and search each clone.

A line note needs the MR's `diff_refs`: `base_sha`, `start_sha`, and `head_sha`. A diff `glab mr diff` can't give comes from the fetched head: `git diff <remote>/<base branch>...refs/pr/<n>`, after `git fetch <remote> <base branch>`.

## Post

| Operation                     | Command                                                                                                                                                                                                                                                                                                             |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Review, line                  | `glab api -X POST projects/<repo>/merge_requests/<n>/discussions -f "position[position_type]=text" -f "position[base_sha]=<base>" -f "position[start_sha]=<start>" -f "position[head_sha]=<sha>" -f "position[old_path]=<file>" -f "position[new_path]=<file>" -F "position[new_line]=<line>" -F body=@<body-file>` |
| Review, file                  | the line call with `position[position_type]=file` and no `new_line`                                                                                                                                                                                                                                                 |
| Summary, or reply to a review | `glab api -X POST projects/<repo>/merge_requests/<n>/notes -F body=@<body-file>`                                                                                                                                                                                                                                    |
| Reply in a thread             | `glab api -X POST projects/<repo>/merge_requests/<n>/discussions/<thread id>/notes -F body=@<body-file>`, the discussion id                                                                                                                                                                                         |
| Description, PR open          | `glab mr update <n> --title "$(cat <title-file>)" --description-file <body-file>`                                                                                                                                                                                                                                   |
| Description, no PR            | `glab mr create --source-branch <branch> --push=false --yes --title "$(cat <title-file>)" --description-file <body-file>`                                                                                                                                                                                           |
| What the post log records     | the note's `id` as `<MR web_url>#note_<id>`, and its `created_at`                                                                                                                                                                                                                                                   |
| Issue, create                 | `glab issue create --title "$(cat <title-file>)" --description-file <body-file> --yes [--label <name>] [--milestone <title>] [--linked-issues <iid>]`                                                                                                                                                               |

Draft notes publish a whole review in one batch. `glab mr create` pushes unless `--push=false` is given, and the kit never pushes.

## Dependencies

| Operation                | Command                                                                                                                                                         |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Open bot MRs             | `glab mr list -R <repo> --per-page 100 -F json`, its `iid`, `title`, `description`, `source_branch`, `sha`, `detailed_merge_status`, `labels`, by branch prefix |
| Mergeable                | `detailed_merge_status` is `mergeable`; conflicted is `conflict` or `need_rebase`                                                                               |
| Security alerts          | Vulnerabilities, below                                                                                                                                          |
| An alert's fixed version | none as a field; `solution` is free text                                                                                                                        |
| Comment on an MR         | `glab mr note create <n> -R <repo> -m <text>`                                                                                                                   |
| Add a label              | `glab mr update <n> -R <repo> --label <label>`                                                                                                                  |
| Wait for CI              | `glab ci status -R <repo> --branch <source branch> --wait`                                                                                                      |
| Merge                    | `glab mr merge <n> -R <repo> --squash --remove-source-branch --sha <sha> --auto-merge=false --yes`                                                              |

`glab mr merge` sets auto-merge unless `--auto-merge=false` is given, and `--sha` refuses a head that moved.

### Vulnerabilities

GitLab Ultimate only; the REST vulnerabilities API is deprecated in favor of GraphQL. `--paginate` needs the `$endCursor` variable and `pageInfo`:

```bash
glab api graphql --paginate -f query='
query($endCursor: String) {
  project(fullPath: "<repo>") {
    vulnerabilities(reportType: [DEPENDENCY_SCANNING], state: [DETECTED, CONFIRMED], first: 100, after: $endCursor) {
      nodes {
        severity title webUrl solution
        primaryIdentifier { name }
        location { ... on VulnerabilityLocationDependencyScanning { file dependency { version package { name } } } }
      }
      pageInfo { hasNextPage endCursor }
    }
  }
}'
```

`state` is `DETECTED`, `CONFIRMED`, `RESOLVED`, or `DISMISSED`; severity is `INFO` through `CRITICAL`. A 403, or a null `project.vulnerabilities`, means the tier or the token can't read them, so report them as "unavailable".
