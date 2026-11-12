# Change

How a code change is sized, shaped, and styled.

## 1. Size the fix to the defect

Applies to bug fixes, patches, and "make X work". Not to new features or to refactors requested as refactors.

- Before a fix, state the defect in one sentence and the minimal change that resolves exactly it.
- An edit larger than that (more files, a new abstraction, an adjacent refactor, bundled cleanup) names the excess and its reason. No reason, ask.

## 2. Follow the existing pattern

- Before a new module, API shape, file, library, state approach, folder shape, or naming convention, read one instance of how this project already solves it and say in one sentence what it showed. Then match it, or name the divergence and its reason; an option that diverges with no reason is ruled out. With no precedent, say there is none. A one-line fix needs no check.
- Before writing code, take the first option that holds. Not needed at all, already in the codebase, the standard library, a native platform feature, an installed dependency, one line, then the minimum new code.
- Before running a script, check the project defines it.

## 3. Name the blast radius before editing shared code

- Before changing a shared component, utility, hook, type, or API contract, list its consumers and state the impact.

<!-- chat: skip -->

- For JS, TS, and Python, `kit blast-radius <file> [symbol]` lists the consumers.

<!-- chat: end -->

- Consumers you can't list quickly get named as a finding before the edit.

## 4. Dead code does not land

- A change leaves no commented-out code, unused import or export, orphaned file, or debug print. Scaffolding requested or marked for the next step is the exception.

## 5. Style

- Match the file's and the project's style, and the framework's idioms.
- Names say what the thing is. No Hungarian notation, no single letters outside loop indices.
- A comment exists only for a hidden constraint, an invariant, a bug workaround, or surprising behavior. One line. Never a restatement of the code or a decorative banner.
- A deliberate shortcut with a known ceiling (a global lock, an O(n^2) scan) gets a `shortcut:` comment naming the ceiling and the upgrade path.
- ASCII box characters appear only in a real diagram.

## 6. Scope

- A feature change refactors no unrelated code and restyles no working code.
- A refactor changes no behavior, so the existing tests pass unchanged. In UI, the look is behavior unless the user asked for a redesign.
- A test that must change during a refactor follows `skills/kit-rules/reference/evidence.md` section 2.
- A task that grows mid-way gets the new scope confirmed first.
- A task bigger than the context can hold gets a proposed split.

## 7. Price the cheapest option

- A list of options includes the cheapest thing that could work (a config or data row, an extended entry, doing nothing). Falls short, rule it out in one line.

## 8. Principles

- KISS: complexity that isn't obvious gets one sentence on why it earns its place.
- YAGNI: no code, config, or abstraction for a call site that doesn't exist yet.
- DRY: duplication along a stable axis gets merged. Copies with divergent lifecycles stay apart.
- SOLID: one reason to change per unit. No interface whose only implementation is its only caller.
- A finding names the principle and its concrete cost.

## 9. What counts as over-engineering

Cut each of these, replaced by the first option in section 2's order that would do:

- Code the standard library, the language, or the platform already provides.
- A dependency doing what a few lines or an installed dependency already do.
- A re-implementation of a helper, type, or pattern already in the codebase.
- An interface with one implementation, a factory with one product, a config option for a value that never changes, a layer with one caller.
- A parameter always passed the same value, an option nothing sets, a branch no caller reaches.
- A wrapper that only forwards, a "for later" stub, code restating what the framework does.

Never cut input validation at a trust boundary, error handling that prevents data loss, security controls, or accessibility.

## 10. Less code is the goal

Applies before writing out a long change, one concept across many files, or a repeated shape, by `skills/engineering-fundamentals/reference/metric-thresholds.md`.

- Take the first that shrinks the code. A language or platform feature, then the pattern `skills/engineering-fundamentals/reference/design-patterns.md` names for the signal, then new code.
- A pattern stays only if it removes more lines than it adds across call sites that exist now.
