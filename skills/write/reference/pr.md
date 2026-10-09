# /write pr

Generate a pull request description from the current diff. Arguments: `<optional: commit range like main..HEAD>`.

## Procedure

1. Check for a project PR template: resolve the project root via `!`kit project-root``, then check these exact paths in order, using the first that exists:
   - `<root>/.github/pull_request_template.md`, and its case variant `<root>/.github/PULL_REQUEST_TEMPLATE.md`
   - the alphabetically-first `*.md` file directly inside `<root>/.github/PULL_REQUEST_TEMPLATE/`, if that directory exists (`fd -H . <root>/.github/PULL_REQUEST_TEMPLATE -e md | sort | head -1`)
   - `<root>/docs/pull_request_template.md`
   - `<root>/pull_request_template.md`

   Test each exact path (e.g. `test -f <path>`) - do not run a keyword/substring search like `fd -i pull_request_template` across the tree. Two failure modes make that unsafe:

   - `.github` is a hidden directory that `fd` silently excludes unless you pass `-H`, so a template living there gets missed entirely.
   - `.github` commonly holds several unrelated `*_pull_request_template.md` variants (per-team or per-change-type templates, e.g. `ds_pull_request_template.md`), and a substring match sorted alphabetically will confidently pick the wrong one instead of the actual default.

   Exact-path checks sidestep both.

   If one exists, read it: its sections replace the default Structure below - this is mandatory, not a preference, even for a one-line diff. Never substitute the default Structure when a project template exists. If none exists, fall back to the default Structure.

2. Resolve the base: !`kit git-base`. If $ARGUMENTS is a valid single-word git ref (no spaces, not a sentence), use it as the explicit base instead by running `kit git-base "$ARGUMENTS"` via Bash.
3. Pull the diff: !`kit git-base --diff`
4. Pull the log (last 20): !`kit git-base --log -20`
5. Read any referenced issue number in recent commit messages, but do not fetch external data.
6. Build the risk map. List the changed files with `kit git-base --diff --name-only` (plus the explicit base from step 2, if any). Run `kit blast-radius <file>` on each one that isn't a test file; it exits 2 for a file type it can't scan, so skip those. A file qualifies when it has 5 or more consumers, or when it has consumers and none of them is a test (`test 0`).

## Output

Write the PR description to the path `kit scratch-dir pr <branch-slug>` prints.

### If step 1 found a project PR template

Reproduce the template verbatim and fill it in. This branch is mandatory and exclusive:

- Every heading in the template appears in the output, spelled and ordered exactly as the template spells and orders it.
- No heading is added. Not `## Summary`, not `## Notes for reviewer`, not anything from the default structure below - that structure does not exist on this branch.
- No heading is removed. A section with nothing to say gets `N/A` or the template's own placeholder, never deletion.
- Non-heading template scaffolding - checklists, HTML comments, instructional italics, blockquotes - is preserved as-is. Tick a checkbox only when the diff establishes it; otherwise leave it unticked.
- The Rules below govern the prose you write _inside_ a section. They never govern which sections exist.
- Risk-map lines from step 6 go in the section closest to reviewer notes, never under a new heading.

### Only if step 1 found no template

Use this default structure:

```
## Summary

<Why this change, now. Lead with motivation or context the diff itself can't convey; do not restate what changed. One short paragraph, no bullet list.>

## Changes

<Grouped by concern, not by file. Concise bullets. Focus on intent, not mechanics.>

## Accessibility

<Only if markup, interaction, or visual change present. Otherwise omit.>

## Testing

<Only manual verification the reviewer cannot already see from the diff or the test files themselves - e.g. env-dependent behavior, a step that needs prod-like data. If the tests in the diff already cover it, omit this section.>

## Risk

<One bullet per file step 6 qualified: `path` - N consumers (S source, T test), and "no test imports it" when T is 0. Omit when nothing qualifies.>

## Notes for reviewer

<Tradeoffs made, intentional omissions, deferred work, uncertainty. Omit if empty.>
```

## Rules

- Why, not what. Hard rule, not a preference: the reviewer already has the diff for what changed and the test files for what's covered. Never restate either. A bullet that only narrates the diff ("renamed X to Y", "added a null check") earns no place in the description - only the bullet's reason for existing does.
- Don't assert an impact or failure mode the diff and commit log don't establish. A one-line diff description ("remove stray character") is not license to invent what would have broken - if the actual runtime consequence isn't in the commit message or a linked issue, describe the change itself and stay silent on severity rather than guessing ("could cause X to fail").
- If a section would only restate the diff or the tests, cut it - default structure only. Under a project template, an empty section keeps its heading and gets `N/A`.
- Shortest description that gives full context. Every sentence should help the reviewer approve faster, not pad the page.
- Tone: direct, professional. Written by the author, not a summarizer. No "Generated by Claude" footers or other AI signatures.
- If the diff is empty, say so and stop.
