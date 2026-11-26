---
name: code-visual
description: Mermaid diagrams of a codebase from its size, imports, churn, and owners, each with its takeaway
argument-hint: "[repo path or subdirectory] [focus: an entry point or flow to trace] [since: git date]"
allowed-tools:
  - Bash(gh pr diff *)
  - Bash(gh pr view *)
  - Bash(glab mr diff *)
  - Bash(glab mr view *)
  - Bash(git log *)
  - Bash(kit code-graph *)
  - Bash(kit git-base *)
  - Bash(kit scratch-dir *)
---

# /code-visual

A visual tour of a codebase: a report of Mermaid diagrams built from its size, imports, churn, and owners, each explained with a takeaway, the why, and what to watch out for.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                                                  | Read                                           |
| ------------------------------------------------------- | ---------------------------------------------- |
| Scoring hotspots or reading churn, Collect steps 1 to 3 | [reference/hotspots.md](reference/hotspots.md) |
| Drawing or editing a `c4-*.mmd` view                    | [reference/c4.md](reference/c4.md)             |
| Drawing any diagram, or fixing a Render failure         | [reference/mermaid.md](reference/mermaid.md)   |

## Inputs

1. Root: `git -C <path or cwd> rev-parse --show-toplevel`. A subdirectory argument narrows every step to it.
2. Report: the path `kit scratch-dir visual <repo-slug>` prints. Raw tool output goes in a sibling folder named after the report, without `.md`. Nothing is written in the repo outside scratch.
3. Read the project CLAUDE.md and README first. Diagrams name things the way the project does.
4. Window: the `since:` argument, any date git's `--since` takes (`6.months`, `2026-01-01`). Without one, ask with AskUserQuestion before collecting: `12.months` (Recommended), `6.months`, `3.months`, all history; "Other" takes a custom date. Every `<since>` below is this value; for all history, drop `--since`.
5. Model folder: `<arch>` is the folder `kit code-graph model-dir <path or cwd>` prints, under kit.yml's `architecture_dir`. A subdirectory gets a scope model of its own: its fingerprint covers only that scope, so it never reads, edits or replaces the whole-repo model.
6. Big projects: when `git -C <root> submodule status` lists submodules, or the first `deps` run finds more than 12 modules, go bottom-up. Dispatch one foreground `general-purpose` agent with `model: sonnet` per module or submodule in parallel, each running this procedure on that subdirectory and drawing it whole, never fanning out again, so each module's model lands in its own scope. Then draw the project's `c4-context.mmd` and `c4-containers.mmd` from those scope models and the cross-module edges in `deps.tsv`, without reading the code again. On a rerun, `kit code-graph check-c4` on each scope says which modules changed; redraw only those, then the project view.

## Collect

1. Size: `tokei <root>` for the language mix; `scc --by-file --format json --no-cocomo <root> > <raw>/scc.json` for per-file code lines and complexity. Without `scc`, skip the hotspot quadrant and say so.
2. History, walked once for churn, activity and coupling: `git -C <root> -c core.quotePath=false log --since=<since> --format='--%H %ad' --date=format:%Y-%m --name-only --relative -- . > <raw>/gitlog.txt`. On a subdirectory run, `<root>` is that subdirectory: `--relative -- .` keeps the history to it, with paths relative to it like scc, ast-grep and `git ls-files` print them. Churn over the window: `rg -v '^(--|$)' <raw>/gitlog.txt | sort | uniq -c | sed -E 's/^ *([0-9]+) (.*)$/\2\t\1/' > <raw>/churn.tsv`.
3. Hotspots: join the two and score complexity x churn, top 10:

```bash
jq -r --arg root "<root>/" --rawfile churn <raw>/churn.tsv '
  ($churn | split("\n") | map(select(length > 0) | split("\t") | {(.[0]): (.[1] | tonumber)}) | add) as $c
  | [.[].Files[] | (.Location | ltrimstr($root)) as $f | {file: $f, complexity: .Complexity, churn: ($c[$f] // 0)}]
  | map(. + {score: (.complexity * .churn)}) | sort_by(-.score) | .[:10][]
  | [.score, .complexity, .churn, .file] | @tsv' <raw>/scc.json
```

4. Dependencies:

```bash
kit code-graph imports <root> > <raw>/imports.tsv
kit code-graph deps <root> <raw>/imports.tsv <raw> [split dir]
```

- When `<arch>/split.txt` names a folder that still exists (`d=$(cat <arch>/split.txt); [ -n "$d" ] && [ -d "<root>/$d" ]`), pass it as the split dir, so a rerun fingerprints the same modules. A missing, empty, or stale file passes nothing, and `deps` finds the split itself.
- Before `deps`, rewrite path aliases with a prefix (`@/x`, `~/x`) to the directory form in `imports.tsv`.
- For a language `imports` doesn't cover, append rows with `rg -n --no-heading` on that language's import keyword, as file and statement text.
- Without ast-grep, `imports` fails naming it. Build every row of `imports.tsv` that way instead, and say in the report's header line that imports come from text matches.
- `deps` prints the split dir and the main source root. Check both before drawing.

5. Owners: `git -C <root> shortlog -sn --no-merges --since=<since> | rg -v '\[bot\]'` for the repo, and the same with `-- <dir>` for each module. Per hotspot: `git -C <root> log --since=<since> --format=%an -- <file> | rg -v '\[bot\]' | sort | uniq -c | sort -rn`; a top author at 70% or more means one person carries that file. Each author named in Who to ask gets their forge handle from one of their pushed commits: on GitHub, `gh api repos/<owner>/<repo>/commits/$(git -C <root> log -1 --no-merges --author='<name>' --format=%H <default branch>) --jq .author.login`. With no forge or no login on the commit, the name stands alone.
6. Activity: `rg '^--' <raw>/gitlog.txt | cut -d' ' -f2 | sort | uniq -c`.
7. Change coupling: `kit code-graph coupling <raw>/gitlog.txt <raw>/imports-resolved.tsv <root> > <raw>/coupling.tsv`. A `no-import` pair breaching the co-change threshold in `skills/engineering-fundamentals/reference/metric-thresholds.md` is hidden coupling: duplicated logic, a list kept in sync by hand, or a registry like a flags file.
8. Read before writing. For each hotspot, upward edge, hidden-coupling pair, and the main flow, read the code and `git -C <root> log --since=<since> --format='%h %s' -- <file> | head -15`. This is where the Why and Watch out prose comes from; a section without it is a chart with a label.

## Diagrams

Directly above every C4 fence from `<arch>`, one provenance line, `Diagram: from <arch>/<file>`; drawn diagrams carry none. At most about 25 nodes per diagram: group or cut before drawing, and follow Readability below. The prose under each diagram follows Report shape.

1. **Context and containers (C4).** The model lives outside the run, in `<arch>` (Inputs step 5). It holds `fingerprint.txt`, `split.txt` (the split dir `deps` printed) and one Mermaid file per view: `c4-context.mmd`, and `c4-containers.mmd` or, when split, `c4-containers-<cluster>.mmd`, each with a same-named `.svg` preview beside it (Render), and `README.md` (Architecture doc). It is reused across runs.
   - Fingerprint: `kit code-graph fingerprint <raw> > <raw>/fingerprint.txt`.
   - Check: `kit code-graph check-c4 <arch> <raw> <root>`. It passes: reuse the files as is and skip to the embed.
   - It lists problems (stale fingerprint lines, a gone path, an edge no import backs, an undeclared package): edit the files in place for each. Keep everything else, since it may hold hand edits. Then copy the new fingerprint and split dir over, and run the check again until it passes.
   - No `c4-*.mmd` files: start from `kit code-graph c4 <raw> <system name> > <arch>/c4-containers.mmd`. Name, merge, or split its containers the way the project talks about them, keeping each container's `%% path: <path>` comment on its own line (Mermaid rejects a trailing comment), relative to `<root>`. Add the `person`, `ext` and `boundary` classDef lines from Colors. Add what imports can't show (people `:::person`, a SaaS no manifest names, `:::ext`), each with `%% source: judgment` on the line under it. Then draw `c4-context.mmd` the same way, and copy the fingerprint and split dir over.
   - The check lists every `judgment` node. Name them in the report's header line, since a person has to confirm those boxes.
   - Embed each file under the init line, with its provenance line above the fence.
   - Say in the report's header line whether the model was reused, updated (and with what), or created.
2. **Shape.** A `flowchart TB` of the modules (the split dir's children inside a subgraph, other top-level directories beside it), each labelled with its main language and code lines.
3. **Dependencies.** A `flowchart LR` from `deps.tsv`, the edge label being the import count. Externals collapse into one node per package, top 5 only. Upward edges from `upward.tsv` are drawn in terracotta with `linkStyle <indexes> stroke:#c8664a,stroke-width:3px`. Under it, a table of upward edges with production imports: edge, production imports and files, one example file.
4. **Hotspots.** A `quadrantChart`, x = complexity and y = churn, each scaled 0 to 1 against the max, top 10 files as points. The top-right quadrant is "refactor first". Write the maximum as `1`: the parser rejects `1.00`. Under it, a table: file, complexity, churn, top owner with their share.
5. **Change coupling.** A `flowchart LR` of the top 8 `no-import` pairs from `coupling.tsv`, nodes named after the file (its folder when the file is `index`), edge label = shared commits, nodes in the `hot` class. Skip it with one line when no pair breaches it.
6. **Main flow**, when a focus is given or the entry point is obvious. A `sequenceDiagram` traced through the code from the entry point, 4 to 8 participants, each named after a real file or module.
7. **Activity.** An `xychart-beta` bar chart of commits per month, plus a `pie` of the language mix.

Diagrams 2 to 7 are drawn. Keep to these diagram types; Render below checks each one parses.

### Readability

- Every fence starts with the init line from Colors. `useMaxWidth: false` makes a large graph render at full size with a scroll, instead of shrinking to the page width. For a C4 view, that line goes above the file's content, which otherwise stays verbatim.
- Large C4 views get split, not shrunk. A container view over 12 nodes becomes 2 or 3 files, each one cluster of containers plus the externals that cluster touches. When this run made the split, the `c4-containers.mmd` it split and its `.svg` are deleted, and the report's header line names each file deleted. A node keeps the same id and label in every file it appears in. Each view is embedded under its own provenance line.
- Node labels run 1 to 4 words. Paths and detail go in the prose, the table, or a C4 file's `%% path:` comment.
- Edges carry a label only when it adds information: a count, a verb that isn't "uses". A C4 view labels every line ([reference/c4.md](reference/c4.md)).

### Colors

One muted palette everywhere: slate blue for the repo's own parts, sage for people, sand for external systems, terracotta for risk.

The init line on every fence. It sets the slate blue every node gets by default, and themes edges, quadrants, pie slices and bars to match:

```text
%%{init: {"theme": "base", "flowchart": {"useMaxWidth": false, "nodeSpacing": 50, "rankSpacing": 70}, "themeVariables": {"fontSize": "18px", "fontFamily": "Inter, system-ui, sans-serif", "primaryColor": "#e8eef6", "primaryTextColor": "#1f3550", "primaryBorderColor": "#4a6fa5", "lineColor": "#7b8794", "secondaryColor": "#e4efe6", "tertiaryColor": "#f4eee5", "quadrant1Fill": "#f6e1d8", "quadrant2Fill": "#f4eee5", "quadrant3Fill": "#eef1f4", "quadrant4Fill": "#e8eef6", "quadrantPointFill": "#c8664a", "pie1": "#4a6fa5", "pie2": "#5b8a6b", "pie3": "#a8875e", "pie4": "#c8664a", "pie5": "#8a7bb0", "pie6": "#5f9ea0", "pie7": "#b0a58f", "xyChart": {"plotColorPalette": "#4a6fa5"}}}}%%
```

In C4 files and drawn flowcharts, mark people with `classDef person fill:#e4efe6,stroke:#5b8a6b,color:#23422d`, external nodes with `classDef ext fill:#f4eee5,stroke:#a8875e,color:#4a3d2c`, C4 system subgraphs with `classDef boundary fill:#ffffff,stroke:#4a6fa5,color:#4a6fa5` (otherwise they take the sand of externals), and hotspot nodes with `classDef hot fill:#f6e1d8,stroke:#c8664a,color:#5a2a1c`.

## Report shape

```markdown
# <project>: visual tour

Root: <root>. <files>, <language mix>. Window: <since> to <date>. Raw data in `<raw folder>`. Model: `<arch>/`, <reused | updated: what changed | created>.

## TL;DR

<3 bullets: what this is, the one part to understand first, where the risk sits>

---

## Context and containers

---

## Shape

---

## Dependencies

---

## Hotspots

---

## Change coupling

---

## Main flow

---

## Activity

---

## Tricky parts

- <non-obvious behavior, file:line, what breaks if you miss it>

---

## Who to ask

- **<dir>**: <author> ([@<handle>](<profile URL>)), <commits> commits
```

Each diagram section is the provenance line for a C4 view, the diagram, its table if the diagram has one, then three labelled parts:

- **Takeaway**: one line, what to conclude, never what the chart shows.
- **Why**: 2 to 4 sentences on why the code is shaped this way or keeps changing, from the code and commit subjects read in Collect step 8. Cite `file:line` or a short commit hash.
- **Watch out**: 1 to 3 bullets on the trap: what bites someone changing this part, and where.

Activity takes only a Takeaway. Tricky parts holds 3 to 6 findings from the reading that fit no diagram: an ordering dependency, a workaround, an implicit contract, a flag that gates a whole area.

## Writing rules

- Explain what, why, and the trap. A why comes from code, a commit message or a PR, and one inferred is labelled per `skills/kit-rules/reference/evidence.md` section 1.
- Readability follows the report-format skill's Readability section. The Takeaway lines alone should tell the story.
- Labels use names from the code.
- Prose and data files (Markdown, JSON, YAML) score 0 complexity and never rank as hotspots. When the busiest area is mostly those, say so in TL;DR.
- A skipped diagram (no clear flow, a single-module repo) gets one line saying why, not a placeholder.

## Render

Before replying, render every diagram: `mkdir -p <raw>/render`, then `mmdc -i <report>.md -o <raw>/render/out.md`. Each chart prints a check or an error with its line. Fix each failing fence in the report, and a failing C4 fence in its `<arch>` file too, then rerun until all pass. Then refresh each `<arch>` file's preview, since editors show a bare `.mmd` as text: `{ echo '<init line>'; cat <arch>/<file>.mmd; } | mmdc -i - -o <arch>/<file>.svg`. If `mmdc` is missing, say the diagrams weren't rendered.

## Architecture doc

The report is a snapshot of one window; `<arch>/README.md` keeps only the structure, so it stays true between runs. Rewrite it every run, after Render passes: the header below, then the report's Context and containers, Shape, and Dependencies sections verbatim. Hotspots, coupling, activity, owners and tricky parts never go in it.

```markdown
# <project>: architecture

Root: <root> at <short HEAD hash>, <date>. Written by /code-visual from the `c4-*.mmd` files and the import graph: edit those files, not this one.
```

## Reply

Report path, `<arch>/README.md`, the TL;DR's first bullet, one next action.

## Scope

Read-only on the repo. Writes only to the scratch report, its raw folder, and `<arch>`.

Arguments: `$ARGUMENTS`
