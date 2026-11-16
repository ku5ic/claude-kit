# Adding a stack

A framework the kit doesn't know needs a pattern skill. A language needs that, plus Go code so the guards and commands can read it. Each step below names the file that owns it; read that file for the how.

## A framework or library

Run `/meta-skill <name>` in a checkout of the kit. It researches the framework from primary sources, drafts `skills/<name>-patterns/`, and wires it into `kit.yml` (`dependency_skills`, `skill_file_map`, `skill_triggers`). Then add the skill's row to [docs/skills.md](skills.md); `go/e2e/kit_repo_test.go` fails until it's there.

## A language

Run `/add-language <name>` in a checkout of the kit. It researches the language and plans the change one step per owner, each with a test that fails first. What it touches:

| What the kit needs to know                                      | Owner                                                                                                                                  |
| --------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| Its manifests, binaries, workspaces, and declared dependencies  | the ecosystem table in `go/internal/sources/deps.go`                                                                                   |
| Its imports, for `kit blast-radius` and `/code-visual`          | `go/internal/codegraph`: the ast-grep rule in `imports.go`, the extensions and families in `importers.go`, the ecosystem in `specs.go` |
| Its comment and string syntax, so the slop guard reads comments | `go/internal/slop/segments.go`                                                                                                         |
| Its package registry, for the new-package guard                 | `kit.yml` `package_lookups`                                                                                                            |
| Its lockfiles and test files                                    | `kit.yml` `lockfile_globs`, and the `test-patterns` globs in `skill_file_map`                                                          |
| Its pattern skill                                               | `/meta-skill`, as for a framework                                                                                                      |

Its checks, fixers, and formatters come from each project's own configs, read by `go/internal/sources`, never from a list in the kit. A task runner the kit can't read yet gets a reader there (`TaskProviders`).
