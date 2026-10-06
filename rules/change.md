# Change

How a code change is sized, shaped, and styled.

## 1. Size the fix to the defect

Before proposing or applying a fix:

1. State the defect in one sentence: what is broken, observed how.
2. Name the minimal change that resolves exactly that, nothing else.
3. Compare it to what you are about to edit. Match -> proceed.
4. Larger -> name the excess and the reason. Excess is: more files touched, a new abstraction, an adjacent refactor, or unrelated cleanup bundled in.
5. Excess with no stated reason -> stop and ask, per `rules/workflow.md` section 4.

"Minimal" is measured against the defect, not against caution. A one-line fix to a one-line bug is minimal. A one-line fix that papers over a genuinely broken abstraction is not - that is the justification to state, not skip.

Applies to bug fixes, patches, and "make X work". Does not apply to net-new features or to refactors requested as refactors; there is no defect to be minimal against.

## 2. Follow the existing pattern, or justify leaving it

Before introducing a new library, state-management approach, folder shape, or naming convention, find how the codebase already solves that class of problem and match it. Divergence is allowed - name and justify it in the change rather than slipping it in.

Before adding a tool, library, or pattern, check what is already in use: `package.json`, the lockfile, existing imports, config files.

Before writing code, take the first option that holds, in this order: not needed at all, already in this codebase, the standard library, a native platform feature (CSS over JS, a DB constraint over app code), an installed dependency, one line, and only then the minimum new code.

Before running a script, check the project defines it: `scripts` in `package.json`, a Makefile, a justfile, a task runner.

## 3. Name the blast radius before editing shared code

Before changing a shared component, utility, hook, type, or API contract, identify the consumers and state the impact. A one-line change to a widely imported module is a wide change wearing a small diff.

Enumerate them with `kit blast-radius <file> [symbol]` (JS/TS and Python imports, test and source counted apart); fall back to `rg` for other languages. If consumers cannot be enumerated quickly, that difficulty is itself a finding to surface before proceeding.

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
- A deliberate shortcut with a known ceiling (a global lock, an O(n^2) scan, a naive heuristic) gets a `shortcut:` comment naming the ceiling and the upgrade path, so `/audit debt` can collect it.
- ASCII box characters (`+`, `-`, `|`, `->`) only when actually drawing a diagram, never as decoration.

## 6. Scope

- Stay in scope. Do not refactor unrelated code as part of a feature change.
- Do not rewrite working code in a different style unless that is the task.
- A refactor, requested or not, changes no behavior: the existing tests pass unchanged before and after. A test that needs changing is a behavior change, per `rules/evidence.md` section 3.
- If the task grows during execution, pause and confirm the expanded scope.
- If a task needs more than the current context can hold, say so and propose a split.

## 7. Price the cheapest option

Whenever options are enumerated - candidate directions, alternatives, approaches - one of them is the cheapest thing that could work: a data, config, or lookup-table row, an existing entry extended, or doing nothing.

It counts as an option even when it is not a code change, and it gets the same scope, risk, and fit treatment as the rest. Insufficient -> say why in one line and rule it out. The failure is never writing it down.

## 8. Principles, with the judgment call named

Each of these is enforced by the judgment it turns on, not by reciting the acronym:

- **KISS**: the simplest sufficient solution wins. Non-obvious complexity gets one sentence saying why it earns its place.
- **YAGNI**: speculative need is not need. No code, config, or abstraction for a call site that does not exist yet.
- **DRY**: duplication along a stable axis is a violation. Duplication whose copies have divergent lifecycles is correct and stays.
- **SOLID**: one reason to change per unit; extend rather than modify working code; no interface whose only implementation is its only caller.

A finding names the principle and the concrete cost. A preference dressed as a principle is not a finding.

## 9. What counts as over-engineering

Writing, reviewing, or simplifying code, these are what to cut, each replaced by the first option in section 2's order that would do:

- **Reinvented stdlib or platform**: hand-rolled code the standard library, the language, or a native platform feature already provides (a deep-clone helper over `structuredClone`, JS layout over CSS).
- **Unneeded dependency**: a package used for what a few lines or an installed dependency already do.
- **Duplicate of existing code**: a helper, type, or pattern re-implemented when one already lives in the codebase.
- **Speculative abstraction**: an interface with one implementation, a factory with one product, a config option for a value that never changes, a layer with one caller.
- **Dead flexibility**: parameters always passed the same value, options nothing sets, branches no caller reaches.
- **Boilerplate**: wrappers that only forward, "for later" stubs, code that restates what the framework does.

Never simplification targets: input validation at trust boundaries, error handling that prevents data loss, security controls, and accessibility. Like any refactor, a simplification changes no behavior (section 6).
