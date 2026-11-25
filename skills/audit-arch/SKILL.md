---
name: audit-arch
description: Judge a codebase's structure against the architecture the project says it has
argument-hint: "<repo path or subdirectory, or none for the whole repo>"
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
---

# /audit-arch

Judges a codebase's structure against what the project says its architecture is. Drawing the structure is `/code-visual`; this audit judges it.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                                  | Read                                                             |
| --------------------------------------- | ---------------------------------------------------------------- |
| Checking the Failure isolation category | [reference/failure-isolation.md](reference/failure-isolation.md) |

## Run

Resolve a linked ticket or doc as `skills/investigate` step 0 describes, then work this file's procedure on the resolved arguments per `skills/kit-rules/reference/subagents.md` section 4, each part in a `kit:auditor` or one auditor for all of it, and write the report yourself. Then run the report-format skill's Runtime checks yourself, and reply per `skills/kit-rules/reference/output.md` section 3.

## Sources

1. Read what the project says it is: its CLAUDE.md, the README's architecture section, `docs/adr/` (or wherever `.adr-dir` points), and any architecture or layering test (`rg -l 'layer|import.*(allow|forbid)' -g '*_test.*'`).
2. The C4 model: `kit code-graph model-dir <target>` prints its folder. When it holds `c4-*.mmd` files, run `kit code-graph check-c4 <model dir> <raw> <root>` after Evidence step 2. A pass: judge against it too. Problems: name them as one `warning`, say the model is stale, and judge without it.
3. None of these exist: judge the structure on its own, and add one `info` finding that the project states no architecture, pointing at `/code-visual` to draw one.
4. The report's TL;DR names the sources it judged against.

## Evidence

1. Raw folder: the path `kit scratch-dir arch <target-slug>` prints, without `.md`.
2. `kit code-graph imports <root> > <raw>/imports.tsv` (without ast-grep, `skills/code-visual/SKILL.md` Collect step 4's fallback, said in the report), then `kit code-graph deps <root> <raw>/imports.tsv <raw> [split dir]`, the split dir from the model dir's `split.txt` as `visual.md` Collect step 4 says, then `git -C <root> -c core.quotePath=false log --since=12.months --format='--%H %ad' --date=format:%Y-%m --name-only --relative -- . > <raw>/gitlog.txt` and `kit code-graph coupling <raw>/gitlog.txt <raw>/imports-resolved.tsv <root> > <raw>/coupling.tsv`.
3. Fan-out per module: `cut -f2 <raw>/deps.tsv | sort | uniq -c`. Fan-in: the same on `-f3`.
4. Read the code behind every candidate before calling it a finding: the import lines of an upward edge, both files of a coupled pair, the call site of an integration.

## Categories

Skip a category with no findings. Each names what proves a finding.

- **Dependency direction and layering.** Proof: a row in `upward.tsv` with production imports, or an import a layering test or ADR forbids. A cycle shows as edges both ways in `deps.tsv`.
- **Module boundaries and cohesion.** Proof: a module whose fan-out reaches most others, or whose files share no imports with each other; a module reaching into another's internals (an import past its public entry).
- **Change coupling.** Proof: a `no-import` pair in `coupling.tsv` breaching the co-change threshold in `skills/engineering-fundamentals/reference/metric-thresholds.md`, and the shared commits that show why.
- **ADR drift.** Proof: an accepted ADR's decision quoted next to code that does otherwise. A superseded ADR is history, not drift.
- **Data and state ownership.** Proof: two modules writing the same table, store, or file, quoted at both write sites.
- **Integration seams.** Proof: one external service called from more than one module with no shared client, found with `rg` for its SDK import or base URL.
- **Failure isolation.** Proof: an external call on a request path with no timeout, retry bound, or fallback, quoted. What each needs: [reference/failure-isolation.md](reference/failure-isolation.md).

Each finding names the pattern `skills/engineering-fundamentals/reference/design-patterns.md` gives for its signal, and the question from `skills/engineering-fundamentals/reference/design-integrity.md` it fails.

## Not a finding

- An upward import only from tests, fixtures, or stories.
- Coupling under that threshold, or between a file and its test.
- A boundary the project's own sources deliberately draw differently than you would.
- Code-level debt inside one module, which `/audit-debt` covers.

## Severity

- `failure`: an import or write that breaks a stated rule (an ADR, a layering test, CLAUDE.md), or a cycle between modules.
- `warning`: an unstated boundary eroding, such as an upward production import, hidden coupling, or a split integration seam.
- `info`: a structure question worth a decision, such as no stated architecture, or a module with very high fan-in.

## Output per finding

- The edge, pair, or module, with `file:line` for each side
- Evidence: the TSV row or the quoted lines
- The source it breaks: the ADR, rule, or test, or "unstated"
- Fix: the first move, and the pattern it applies

## Output file

Load the report-format skill and use its format. Write to the path `kit scratch-dir arch <target-slug>` prints.

Arguments: `$ARGUMENTS`
