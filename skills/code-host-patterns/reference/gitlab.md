# GitLab

The `glab` CLI. `<repo>` is `group/project`, URL-encoded (`/` as `%2F`) in an API path; every `glab mr` call passes `-R <repo>`.

## Read

| Operation                    | Command                                                                                                                |
| ---------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| Repository path              | `glab repo view -F json`, its `path_with_namespace`                                                                    |
| Current user                 | `glab api user`, its `username`                                                                                        |
| The current branch's open PR | `glab mr view -F json`, its `iid` and `web_url`                                                                        |
| Head SHA, title, body, files | `glab mr view <n> -F json`: `sha`, `source_branch`, `title`, `description`, `diff_refs`                                |
| Diff                         | `glab mr diff <n>`                                                                                                     |
| Head ref to fetch            | `+merge-requests/<n>/head:refs/pr/<n>`                                                                                 |
| Review threads               | `glab api projects/<repo>/merge_requests/<n>/discussions --paginate`; a diff note's `position` holds its path and line |
| Link to a line at the head   | `<project web_url>/-/blob/<sha>/<path>#L<line>`                                                                        |
| PR template                  | `.gitlab/merge_request_templates/Default.md`, any letter case                                                          |

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

Draft notes publish a whole review in one batch. `glab mr create` pushes unless `--push=false` is given, and the kit never pushes.
