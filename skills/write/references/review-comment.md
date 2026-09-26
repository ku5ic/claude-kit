# /write review-comment

Convert a structured review report into a peer-to-peer GitHub PR comment. Arguments: `[path to review report] [PR author username]`.

## Procedure

1. Get the scratch directory: `!`scratch-dir.sh``.
2. Parse $ARGUMENTS. If the first token resolves to an existing file, treat it as the report path and any remaining token as the PR author username. Otherwise treat the whole of $ARGUMENTS as the PR author username and leave the report path unset.
3. If no report path was resolved, stop and ask for one: any file in the report-format skill's shape, e.g. saved `/code-review` output or an `/audit` report (forked: follow the forked decision protocol in `rules/agents.md` instead of guessing).
4. Read the review report. It follows the report-format skill's format: a severity rubric (failure/warning/info), each finding with file, line, "What", "Why it matters", and "Fix".
5. Look for a PR number in the report's `Scope:` line, its filename, or body (patterns like `pr-123`, `PR #123`, `PR: 123`). If found, run `gh pr view <n> --json author,headRefOid`. Use `.author.login` as the auto-detected author and `.headRefOid` as the ref for links.
6. Resolve the repo slug: `gh repo view --json nameWithOwner -q .nameWithOwner`. If this fails (no remote, no auth), fall back to plain backticked `path:line` text for every finding - no links - and say so at the top of the comment.
7. Resolve the ref for links: the PR's head sha from step 5 if a PR was found; otherwise the current commit, `git rev-parse HEAD`.
8. Pull the summary line and every failure/warning finding. Include an info finding only if it is a quick win or visibly affects code quality; drop the rest.
9. For each kept finding, decide actionable ("real bug", "worth fixing") vs. non-issue ("feel free to drop"), based on severity and the "Why it matters" text, not guesswork.
10. Resolve the @mention: an explicit username argument wins; otherwise the gh-detected PR author; otherwise `@author` as a placeholder, flagged as needing a fill-in.

## Building file links

For every finding, build a real link instead of relying on GitHub to auto-linkify bare text - auto-linkify only works for paths that are relative to the repo root and present at the linked ref, so it silently fails for absolute paths or paths copied from a report generated in a different working directory.

1. Resolve the finding's path to one relative to the repo root: strip any absolute prefix that falls inside the working tree (e.g. `/Users/x/project/src/y.ts` inside repo `project` becomes `src/y.ts`). This covers "outside of repo" references the same way as already-relative ones - there is only one link-building path, not a special case.
2. Build the URL based on the location shape:
   - Single line: `https://github.com/<repo-slug>/blob/<ref>/<relative-path>#L<line>`.
   - Range `a-b`: `#La-Lb`.
   - Non-numeric region (e.g. "constructor"): link to the file with no line anchor and keep the region name as text.
3. If the path cannot be resolved inside the repo at all (points to something genuinely outside this working tree), keep it as backticked `path:line` text with no link, and flag it as unresolved rather than guessing a URL.
4. Skip this entirely if step 6 could not resolve a repo slug.

## Output file

Write to the path `scratch-dir.sh review-comment <scope-slug>` prints. Print the path. `<scope-slug>` comes from the input report's `Scope:` line if present, otherwise from the input filename.

Structure (GitHub markdown, no frontmatter, no metadata - copy-paste ready as a single PR comment):

```
Hey @<author>, <genuine one-line compliment about the work>.

<1-3 actionable issues, each as:>
- [path:line](link) - plain-language description of the problem, framed as "this one's a real bug" or "worth fixing".

<Non-issues, if any, framed as "feel free to drop this one" - grouped separately from the actionable issues, not interleaved.>

<Closing: a concrete next step or an open question. Never "let me know if you have questions".>
```

## Rules

- Tone: peer to peer, professional, warm. Never "I recommend", "you should", or "let me know if you have questions".
- Every finding link points at the exact line(s) from the source report, unedited.
- Skip info-level findings unless they are quick wins or visibly affect code quality.
- The comment is read in a PR thread, so it stays scannable in one screen and covers only the findings worth the author's time. If it grows past what one comment can carry, split it in the same output file, each in its own fenced block, labeled "Comment 1 of N" etc.
- Describe the problem and point at the fix; do not rewrite the actual code fix.
- Do not re-run the review or invent findings not present in the source report.
- The latest-report fallback in step 3 only ever searches the resolved scratch directory for the current project; never fall back to another project's file.
- If the resolved input file does not exist or does not match the report-format skill's format, say so and stop.
