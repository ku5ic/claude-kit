<img src="assets/claude-kit-mark.svg" alt="" width="40">

# Skills Claude loads on its own

Without these, Claude gives generic advice that ignores your stack and your project's practice. Each skill is a short SKILL.md that points at reference files, and a "Before | Read" table says which file to read before which action, so Claude reads React's correctness reference before writing a hook, and its performance reference only when memoizing.

## When they load

- **Session start** names the skills your repo needs, from the files it holds (`kit.yml` `skill_file_map`) and the dependencies its manifests declare (`dependency_skills`).
- **The first read or edit** of a file type names its skill again.
- **`guard_skills: true`** makes a file type's skill required before its first edit ([configuration](configuration.md)).
- **`kit-rules`** is required: every session and subagent loads it before any other tool call.

Each stack skill's guidance comes from primary sources (official docs, changelogs, specs), listed with the date they were checked in its `sources.md`. `/meta-refresh` checks them against current releases.

## Working skills

| Skill                      | What it covers                                                                              | Loads                                                                |
| -------------------------- | ------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| `kit-rules`                | The kit's rules: reply length, evidence, change size, git, verification, subagents, tooling | First, in every session and subagent                                 |
| `investigate`              | A read-only answer to how, why, or where, or a symptom traced to its cause                  | When you ask a question or report a bug without asking for a change  |
| `engineering-fundamentals` | Design and code checks: modularity, size, nesting, naming, errors, red-green-refactor       | Before planning a change or editing code, in any language            |
| `test-patterns`            | Test design, fixtures, mocking, async, coverage, per runner                                 | Before editing a test, helper, fixture, or test config               |
| `git-patterns`             | Conventional Commits, rebase, reflog recovery, bisect, worktrees                            | Before a commit, rebase, or other git operation                      |
| `security-patterns`        | XSS, injection, auth, sessions, CSRF, CSP, secrets, CVEs                                    | Before code that handles auth, secrets, or user input                |
| `slop-patterns`            | How to spot AI tells and filler, and rewrite them around a fact                             | When a guard flags AI tells, or before drafting a commit, PR, or doc |
| `report-format`            | The shape and severity scale of audit, review, and dependency reports                       | Before writing a report                                              |
| `code-host-patterns`       | GitHub and GitLab CLI and API calls for PRs, MRs, reviews, issues                           | Before a `gh` or `glab` call                                         |
| `wcag-audit`               | The WCAG 2.2 AA checklist and severity rubric                                               | For accessibility questions or an accessibility audit                |

## Stack skills

| Skill                      | What it covers                                                                    | Loads before an edit to                                     |
| -------------------------- | --------------------------------------------------------------------------------- | ----------------------------------------------------------- |
| `react-patterns`           | Hooks rules, component design, rendering performance                              | `.jsx`, `.tsx`                                              |
| `next-app-router-patterns` | The server and client boundary, caching, server actions, streaming, runtimes      | `app/`, `next.config.*`                                     |
| `vue-patterns`             | The Composition API, reactivity, single-file components                           | `.vue`, a composable                                        |
| `nuxt-patterns`            | Rendering modes, `useFetch` and `useAsyncData`, server routes, SSR state, Nitro   | a Nuxt app, `nuxt.config.*`                                 |
| `typescript-patterns`      | Strictness, `any` vs `unknown`, `satisfies`, unions, generics, module boundaries  | `.ts`, `.tsx`, `tsconfig`                                   |
| `javascript-patterns`      | Modern syntax, async, ESM and CJS, errors                                         | `.js`, `.mjs`, `.cjs`, `.jsx`, `package.json`               |
| `tailwind-patterns`        | Tailwind v4 `@theme` config, v3-to-v4 changes, variants                           | a class list, a stylesheet, `tailwind.config.*`             |
| `storybook-patterns`       | CSF 3, args and `fn()` spies, play functions, story ids, the a11y addon           | `*.stories.*`, `.storybook/`                                |
| `graphql-patterns`         | Operations, fragments, the codegen client preset, Apollo Client 4                 | `.graphql`, `.gql`                                          |
| `python-patterns`          | Typing, project layout, async, errors, packaging                                  | `.py`, `pyproject.toml`                                     |
| `django-patterns`          | The ORM, views, migrations, settings                                              | a Django app                                                |
| `drf-patterns`             | Serializers, viewsets, permissions, throttling                                    | a serializer, viewset, or API view                          |
| `fastapi-patterns`         | `Depends`, response models, async, errors, auth, with Pydantic v2                 | a file importing `fastapi`                                  |
| `logging-patterns`         | Python stdlib logging and structlog: config, levels, handlers, JSON logs          | Python logging code                                         |
| `monitoring-patterns`      | Prometheus client metric types, naming, labels, health checks                     | Python metrics or health-check code                         |
| `go-patterns`              | Errors, context, goroutines, interfaces, recent release features, tests           | `.go`, `go.mod`                                             |
| `bash-patterns`            | Safety flags, quoting, traps, ShellCheck                                          | `.sh`, `.bash`, `.zsh`, a shebang script, a Makefile recipe |
| `docker-patterns`          | Dockerfiles, multi-stage builds, Compose, networking, volumes                     | a Dockerfile, a Compose file, `.dockerignore`               |
| `opentofu-patterns`        | `moved`, `import`, and `removed` blocks, `for_each`, state secrets, the lock file | `.tf`, `.tfvars`                                            |
| `vps-provisioning`         | Server setup, firewall, nginx reverse proxy, Let's Encrypt, systemd, hardening    | a playbook, a provisioning script, nginx or systemd config  |
| `backup-patterns`          | The 3-2-1 rule, rsync, `pg_dump`, restic, retention, restore tests                | a backup script or job                                      |

Your stack has none: `/meta-skill` drafts one from current docs and your repo's precedent ([commands](commands.md)).
