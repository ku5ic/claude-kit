# Change

How a code change is sized, shaped, and styled.

## 1. Size the fix to the defect

Before proposing or applying a fix:

1. State the defect in one sentence: what is broken, observed how.
2. Name the minimal change that resolves exactly that, nothing else.
3. Compare it to what you are about to edit. Match -> proceed.
4. Larger -> name the excess and the reason. Excess is: more files touched, a new abstraction, an adjacent refactor, or unrelated cleanup bundled in.
5. Excess with no stated reason -> stop and ask.

"Minimal" is measured against the defect, not against caution. A one-line fix to a one-line bug is minimal. A one-line fix that papers over a genuinely broken abstraction is not - that is the justification to state, not skip.

Applies to bug fixes, patches, and "make X work". Does not apply to net-new features or to refactors requested as refactors; there is no defect to be minimal against.

## 2. Follow the existing pattern, or justify leaving it

Before introducing a new library, state-management approach, folder shape, or naming convention, find how the codebase already solves that class of problem and match it. Divergence is allowed - name and justify it in the change rather than slipping it in.

Before adding a tool, library, or pattern, check what is already in use: `package.json`, the lockfile, existing imports, config files.

Before running a script, check the project defines it: `scripts` in `package.json`, a Makefile, a justfile, a task runner.

## 3. Name the blast radius before editing shared code

Before changing a shared component, utility, hook, type, or API contract, identify the consumers and state the impact. A one-line change to a widely imported module is a wide change wearing a small diff.

Enumerate them with `blast-radius.sh <file> [symbol]` (JS/TS and Python imports, test and source counted apart); fall back to `rg` for other languages. If consumers cannot be enumerated quickly, that difficulty is itself a finding to surface before proceeding.

## 4. Dead code does not land

A change leaves behind no commented-out blocks, no unused imports, no unreferenced exports, no orphaned files, no leftover console or print debugging. Removed behavior means removed code; version control holds the history.

The one exception is scaffolding explicitly requested or explicitly marked for a following step.

## 5. Style

- Match the existing style of the file and the project. If Prettier, ESLint, Biome, or similar config exists, conform to it.
- Prefer idiomatic patterns for the framework in use over generic ones.
- Meaningful names. No Hungarian notation. No single-letter variables except loop indices.
- Readability and explicitness over cleverness.

### Comments

- Comment only what is not obvious: a hidden constraint, a subtle invariant, a workaround for a specific bug, behavior that would surprise a reader.
- If removing the comment would not confuse a future reader, do not write it.
- One line. A "why" needing a paragraph belongs in the commit message or PR description.
- Remove comments that restate the code.
- No decorative comments: no banners, dividers, or headers made of `===`, `---`, `***`, `###`.
- ASCII box characters (`+`, `-`, `|`, `->`) only when actually drawing a diagram, never as decoration.

## 6. Scope

- Stay in scope. Do not refactor unrelated code as part of a feature change.
- Do not rewrite working code in a different style unless that is the task.
- If the task grows during execution, pause and confirm the expanded scope.
- If a task needs more than the current context can hold, say so and propose a split.

## 7. Price the cheapest option

Whenever options are enumerated - candidate directions, alternatives, approaches - one slot belongs to the cheapest thing that could work: a data, config, or lookup-table row, an existing entry extended, or doing nothing.

It counts as an option even when it is not a code change, and it gets the same scope, risk, and fit treatment as the rest. Insufficient -> say why in one line and rule it out. The failure is never writing it down.

## 8. Principles, with the judgment call named

Each of these is enforced by the judgment it turns on, not by reciting the acronym:

- **KISS**: the simplest sufficient solution wins. Non-obvious complexity gets one sentence saying why it earns its place.
- **YAGNI**: speculative need is not need. No code, config, or abstraction for a call site that does not exist yet.
- **DRY**: duplication along a stable axis is a violation. Duplication whose copies have divergent lifecycles is correct and stays.
- **SOLID**: one reason to change per unit; extend rather than modify working code; no interface whose only implementation is its only caller.

A finding names the principle and the concrete cost. A preference dressed as a principle is not a finding.

Simplicity and unnecessary-abstraction avoidance is also enforced by the `ponytail` plugin, enabled globally.

## Anti-patterns

- `failure`: touching files unrelated to the defect without naming why.
- `failure`: a new abstraction (interface, config layer, helper module) for a single call site, without stating why now rather than at the second call site.
- `warning`: refactoring surrounding code "while in there" during a bug fix.
- `warning`: describing a fix as "cleanup" when the ask was a specific defect.
- `warning`: an option list with no cheapest option priced, or one mentioned in passing instead of ruled out.
- `info`: a fix smaller than expected because the defect was smaller than it looked. Worth noting, not a violation.
