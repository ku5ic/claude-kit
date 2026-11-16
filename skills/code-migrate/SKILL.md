---
name: code-migrate
description: Inventory every use of what a migration replaces, in one repo or across an org, and batch it into plan steps with codemods
argument-hint: "<target: package@version, API, or pattern> [org:<org or group>]"
disable-model-invocation: true
allowed-tools:
  - Bash(gh search code *)
  - Bash(git log *)
  - Bash(kit detect-stack)
  - Bash(kit scratch-dir *)
  - Bash(sg run *)
---

# /code-migrate

Find every use of what a migration replaces (a major upgrade, a renamed API, a deprecated pattern) and cut the work into batches that each ship alone, with the codemod for each where one exists. The report feeds `/plan`. Read-only: it previews a codemod, never applies one.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                                              | Read                                                 |
| --------------------------------------------------- | ---------------------------------------------------- |
| Step 2, reading the upgrade guide and the changelog | [reference/deprecation.md](reference/deprecation.md) |
| Step 3, the inventory                               | [reference/inventory.md](reference/inventory.md)     |
| Step 4, choosing a codemod and previewing it        | [reference/codemods.md](reference/codemods.md)       |
| Step 5, cutting batches                             | [reference/batching.md](reference/batching.md)       |

Stack specifics come from the stack's pattern skill.

## Procedure

1. Inputs. The target and its from and to versions come from the arguments, else the manifests `kit detect-stack` names. Unclear, ask. `org:` switches to the org mode below.
2. What changes. Read the upgrade guide and the changelog between the two versions, per [reference/deprecation.md](reference/deprecation.md): each removed or renamed API, the version that deprecated it, the version that removes it, and the vendor's codemods. A source that can't be fetched is named in the report, never filled from memory.
3. Inventory. One `sg` pattern per API from step 2, and `rg -w` for what a pattern can't parse, per [reference/inventory.md](reference/inventory.md). Raw matches go to the report's sibling folder, one TSV row per match with the API, file, and line.
4. Codemods. For each API, the vendor's codemod, else an `sg` rewrite. Preview it only with a tool already installed, and count the files it would change and the spots it leaves for hand work.
5. Batches. Cut the inventory into batches per [reference/batching.md](reference/batching.md), each one plan step with its files, its codemod or hand edits, and the check that proves it (a test command, a type check, a zero count from step 3's pattern).

## Org mode

With `org:<name>`, the inventory runs across the org's repos through the host's code search, by the code-host-patterns skill's "Code search across an org" row. Its limits (default branch only, a result cap, a rate limit) go in the report's Caveats.

- Code search gives a lower bound. Each repo with hits gets step 3 on a shallow clone in the report's sibling folder before its count is trusted.
- One batch per repo, in the order the repos' counts and step 2's removal versions suggest.
- A rerun at the same path reads the previous report and adds a Progress column per repo, uses left now against uses at the first run. The total counts across all repos.

## Report

Load the report-format skill and use its format, written to the path `kit scratch-dir migrate <target-slug>` prints, with these narrowings:

- The TL;DR's first bullet is the size in place of the health word: `<N> uses in <M> files[ across <R> repos], <B> batches, <C> covered by codemods`.
- A Batches table with the batch, files, uses, codemod or `by hand`, and the proving check.
- Org mode adds a Repos table with the repo, uses, batch, and Progress on a rerun.
- Caveats list the uses a pattern can't see (strings, config, generated code), the APIs with no known removal version, and the code-search limits.

## Reply

The report path and the TL;DR's first bullet.

## Stop

Read-only on every repo. No codemod applied, no install, no `npx` of a package that isn't installed, no branch or PR.

Arguments: `$ARGUMENTS`
