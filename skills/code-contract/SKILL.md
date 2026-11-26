---
name: code-contract
description: What a change does to a public surface, each change classed patch, minor, or major, with the consumers it reaches
argument-hint: "[base ref] [path]"
allowed-tools:
  - Bash(git diff *)
  - Bash(git log *)
  - Bash(git show *)
  - Bash(kit code-graph *)
  - Bash(kit detect-stack)
  - Bash(kit git-base *)
  - Bash(kit scratch-dir *)
  - Bash(ast-grep run *)
---

# /code-contract

Diff the public surface between a base and the working tree, class each change patch, minor, or major, and list what each change reaches. Read-only.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                                          | Read                                                                                           |
| ----------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Step 2, with a schema file                      | [reference/schemas.md](reference/schemas.md)                                                   |
| Step 2, with the ecosystem's API tool installed | [reference/ecosystem-tools.md](reference/ecosystem-tools.md)                                   |
| Step 3, classing each change                    | [reference/semver.md](reference/semver.md), [reference/behavioral.md](reference/behavioral.md) |

Stack specifics come from the stack's pattern skill. The commit-message bump rules are the git-patterns skill's.

## Procedure

1. Surface. The base is the argument, else `kit git-base`. A path argument limits the surface to what's under it. What's public comes from the project: a package's declared exports, a published schema, its routes, a component library's props. The report names the surface it treated as public. Unclear, ask.
2. Base and head, first that applies:
   - A schema file (OpenAPI, GraphQL, protobuf). Its checker runs on the base's copy (`git show <base>:<file>`) and the working tree's, per [reference/schemas.md](reference/schemas.md).
   - The ecosystem's API tool, installed and configured in the project, run per [reference/ecosystem-tools.md](reference/ecosystem-tools.md).
   - Otherwise exported symbols, routes, and props, found with `ast-grep` on both sides, the base's files read with `git show`. The report says no checker ran.
3. Class each change by [reference/semver.md](reference/semver.md), then scan the diff for what [reference/behavioral.md](reference/behavioral.md) lists, which no type diff sees.
4. Consumers. Each changed symbol gets `/code-impact`'s Consumers procedure, run here. A published package's consumers outside the repo are unknown, and the report says so.

## Report

Load the report-format skill and use its format, written to the path `kit scratch-dir contract <scope-slug>` prints, with these narrowings:

- The TL;DR's first bullet is the bump in place of the health word: `patch`, `minor`, or `major: <the change that forces it>`. Under `0.y.z`, it says the class is advice.
- A Changes table with the change, its class, its evidence (the checker's line, or `file:line` on each side), and its consumers.
- A major change is a `failure` when the project's version or commit doesn't say major, else `info`. A behavioral change is a `warning`. A change that is both takes the higher.

## Reply

The report path and the bump line.

## Stop

Read-only. No version bump, no changelog edit, no install.

Arguments: `$ARGUMENTS`
