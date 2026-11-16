# git mechanics that skew a history

- `--follow` follows renames for a single file only. A folder's history stops at a rename; say so.
- `--no-merges` drops merge commits. Without it, merges count as commits.
- `--numstat` prints `-` for both counts of a binary file. Skip those before ranking.
- Rename detection defaults to 50% similarity (`-M<n>%`); `-M100%` finds exact renames only.
- `-S<string>` finds commits that change how many times a string occurs, so a line edited in place is missed. `-G<regex>` matches any added or removed line.
- `--since` stops walking at the first commit older than the date. `--since-as-filter` checks every commit instead.
- GitHub GraphQL: a connection takes `first` or `last` from 1 to 100, and one call is capped at 500,000 nodes and 10 seconds. A timeout returns 502 or 504.
