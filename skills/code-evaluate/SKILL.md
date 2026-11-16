---
name: code-evaluate
description: Whether to adopt a package, or which of several: need, health, security, license, footprint, fit, and exit cost
argument-hint: "<package[@version]> [vs <package> ...] [ecosystem]"
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
  - Bash(kit detect-stack)
  - Bash(kit scratch-dir *)
---

# /code-evaluate

Evaluate a package before it's adopted, or compare candidates, the way a reviewer would: whether it's needed at all, whether it's healthy and safe, and what leaving it later would cost. Read-only; installs nothing.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                                      | Read                                               |
| ------------------------------------------- | -------------------------------------------------- |
| Step 3, the assessment                      | [reference/assessment.md](reference/assessment.md) |
| Step 2, querying the registries and scorers | [reference/data.md](reference/data.md)             |

Supply-chain checks also follow security-patterns' `reference/dependencies.md`, and the ecosystem's own conventions its pattern skill.

## Procedure

1. Inputs. No package named: ask. The ecosystem comes from the argument, else from `kit detect-stack` and the manifest the package would join; unclear, ask. A version not given is the latest stable.
2. Collect, per candidate, from [reference/data.md](reference/data.md): the version's license, advisories, publish date, and dependencies; the project's Scorecard; the release history. Save raw responses in the report's sibling folder. A source that fails gets named in the report, never filled from memory.
3. Assess each candidate against [reference/assessment.md](reference/assessment.md), in its order. Necessity comes first: what the repo, the standard library, or a dependency already installed does today, found with `rg` on the repo and in its manifests. That's the cheapest option (`skills/kit-rules/reference/change.md` section 7).
4. Fit: read how the repo would call it. The two or three call sites it would replace or add, by `file:line`, and whether its API covers them without wrappers.
5. Exit cost: how many files would import it, and whether its types or concepts would leak past one module.

## Report

Load the report-format skill and use its format, written to the path `kit scratch-dir evaluate <package-slug>` prints, with these narrowings:

- The TL;DR's first bullet is the verdict in place of the health word: `adopt`, `adopt with: <condition>`, `don't adopt: <reason>`, or `use <alternative>`.
- With several candidates, a table first: one row each, one column per assessment area, each cell a value (a date, a count, a license id, a score).
- Each finding names the assessment area, its evidence (the API field and value, or the `file:line`), and its source.

## Reply

The report path and the verdict line.

## Stop

Read-only: installs, adds, or runs nothing from the package.

Arguments: `$ARGUMENTS`
