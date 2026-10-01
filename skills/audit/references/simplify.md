# /audit simplify

Find what to delete or replace with something smaller: over-engineering only, not bugs. Arguments: `<diff (the branch against its base), a file, a directory, or empty for the whole repo>`.

## Procedure

1. Stack is in the `<repo-context>` block from the `SubagentStart` hook. Get the scratch directory via `scratch-dir.sh`.
2. Load the patterns skill for the detected stack for its idioms and stdlib reference.
3. Scope:
   - `diff`: `git-base.sh --diff`. Judge only added or changed code; surrounding code is context.
   - A path: read it. For a directory, `tokei --sort code <path>` and read the 5 largest files plus the entry points.
   - Empty: the repo root, same as a directory.
4. Read the dependency manifest (`package.json`, `pyproject.toml`, `go.mod`, or similar) for what is already installed.
5. Climb the ladder in `rules/change.md` section 2 for each candidate: the finding is the lowest rung that would replace it. Skip categories with no findings. Do not pad.

### Categories

- **Reinvented stdlib or platform**: hand-rolled code the standard library, the language, or a native platform feature already provides (a date picker over `<input type="date">`, a deep-clone helper over `structuredClone`, JS layout over CSS)
- **Unneeded dependency**: a package used for what a few lines or an installed dependency already do
- **Duplicate of existing code**: a helper, type, or pattern re-implemented when one already lives in the codebase
- **Speculative abstraction**: an interface with one implementation, a factory with one product, a config option for a value that never changes, a layer with one caller
- **Dead flexibility**: parameters always passed the same value, options nothing sets, branches no caller reaches
- **Boilerplate and scaffolding**: wrappers that only forward, "for later" stubs, code that restates what the framework does

Out of scope: correctness, security, and style. Input validation at trust boundaries, error handling that prevents data loss, security controls, and accessibility are never simplification targets.

## Output per finding

- Location: `file:line`
- What to cut
- What replaces it (the stdlib call, the native feature, the existing helper, or nothing), with the rung it sits on
- Lines saved, approximately
- Severity: warning when it adds a dependency or an abstraction layer, info otherwise (per the report-format skill's rubric)

## Output file

Load the report-format skill and use its format. Write to the path `scratch-dir.sh simplify <target-slug>` prints. Print the path.

Sort findings by lines saved, largest first.
