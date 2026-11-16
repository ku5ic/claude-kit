# code-host-patterns sources

The sources and version notes behind this skill, for `/meta-refresh`. Nothing links here, so a session never loads it.

## reference/github.md

### References

- gh manual: https://cli.github.com/manual/
- Pull request review comments: https://docs.github.com/en/rest/pulls/comments
- Issue comments: https://docs.github.com/en/rest/issues/comments
- Rate limits: https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
- Pull request templates: https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/creating-a-pull-request-template-for-your-repository
- Dependabot alerts: https://docs.github.com/en/rest/dependabot/alerts

## reference/gitlab.md

### References

- glab commands: https://gitlab.com/gitlab-org/cli/-/tree/main/docs/source
- Discussions API, diff notes and replies: https://docs.gitlab.com/api/discussions/
- Notes API: https://docs.gitlab.com/api/notes/
- Draft notes: https://docs.gitlab.com/api/draft_notes/
- Merge request refs: https://docs.gitlab.com/user/project/merge_requests/merge_request_troubleshooting/
- Description templates: https://docs.gitlab.com/user/project/description_templates/
- Merge requests API, `detailed_merge_status`: https://docs.gitlab.com/api/merge_requests/
- Vulnerabilities, REST deprecated for GraphQL: https://docs.gitlab.com/api/vulnerabilities/
- GraphQL `Project.vulnerabilities`: https://docs.gitlab.com/api/graphql/reference/

### Version notes

Checked: 2026-11-16 against the glab docs at https://gitlab.com/gitlab-org/cli/-/raw/main/docs/source and the GitLab docs source at https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc

- glab `mr create` pushes by default with `--fill`; `--push=false` skips it.
- glab `mr merge` sets `--auto-merge` by default; `--auto-merge=false` turns it off.
- glab `api graphql --paginate` needs an `$endCursor` variable and `pageInfo { hasNextPage endCursor }`.
