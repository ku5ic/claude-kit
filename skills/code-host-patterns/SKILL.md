---
name: code-host-patterns
description: GitHub and GitLab CLI and API calls for PRs and MRs, review threads, comments, head refs, and PR templates. Load before any gh or glab call, however small.
---

# Code host patterns

The commands behind each PR or MR operation, per code host. Which host a repo is on: `skills/kit-rules/reference/tooling.md` section 4. A host with neither CLI: the matching operation of its MCP connector, or say none was found.

A skill names an operation (the PR's head SHA, a line comment, the PR template), and the host's file gives its command.

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                       | Read                                       |
| -------------------------------------------- | ------------------------------------------ |
| A `gh` call, or any PR operation on GitHub   | [reference/github.md](reference/github.md) |
| A `glab` call, or any MR operation on GitLab | [reference/gitlab.md](reference/gitlab.md) |
