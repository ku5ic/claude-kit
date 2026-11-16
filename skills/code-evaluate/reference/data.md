# Data sources

Where each fact comes from, without an install. Every call is a GET unless named; save each response raw.

## deps.dev

`https://api.deps.dev/v3`, systems `NPM`, `PYPI`, `GO`, `MAVEN`, `CARGO`, `RUBYGEMS`, `NUGET`. A package name with `/` or `@` is URL-encoded. A related project's `relationProvenance` of `UNVERIFIED_METADATA` means the package only claims that repo, which the authenticity check has to confirm.

| Fact                                              | Call                                                                                                                                       |
| ------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| Versions, each with `publishedAt`, `isDeprecated` | `/systems/<system>/packages/<name>`                                                                                                        |
| A version's `licenses`, `advisoryKeys`, links     | `/systems/<system>/packages/<name>/versions/<version>`                                                                                     |
| Its resolved dependency graph                     | the version call, plus `:dependencies`                                                                                                     |
| The source project's Scorecard and stars          | `/projects/<project id>`, URL-encoded: the `relatedProjects` entry whose `relationType` is `SOURCE_REPO`, like `github.com/<owner>/<repo>` |

## OSV

`POST https://api.osv.dev/v1/query` with `{"package": {"name": "<name>", "ecosystem": "<ecosystem>"}, "version": "<version>"}`. Each vuln's `affected[].ranges[].events` holds `introduced`, `fixed`, and `last_affected`; `fixed` is the version with the fix, and `last_affected` caps a range with none. Give the version once: in the field, or in a purl, never both.

## Scorecard

`https://api.scorecard.dev/projects/github.com/<owner>/<repo>`: `score`, and `checks[]` with each `name`, `score`, and `reason`. No result means the project was never scored, and the report says so.

## The project itself

Release history and maintainers from the repo's tags and contributors, through the code host (`skills/kit-rules/reference/tooling.md` section 4) when it's GitHub or GitLab, else from the registry's version list.
