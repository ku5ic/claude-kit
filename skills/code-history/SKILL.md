---
name: code-history
description: A file or folder's git and PR history: turning points, recurring bugs, broken promises, who to ask
argument-hint: "<file or folder path, or a component name>"
disable-model-invocation: true
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

# /code-history

The git and PR history of a file or folder, as a report with turning points, recurring bugs, broken promises, who to ask, and a short story. With no argument, ask for the path before anything else.

## Inputs

1. Resolve the argument to a real path. A loose name ("Filter ds component") may not match one: search with `fd` and `rg`, pick the match, and state the substitution in the report header.
2. Size the job: `git log --format='%h' -- <path> | wc -l`.
3. Find the code host per `skills/kit-rules/reference/tooling.md` section 4, and resolve the repo's path and default branch through it. With no CLI or connector for the host, say so and work from commits alone.
4. A docs or decision-log connector, found the same way, is optional. If one exists, search it for the component name and record whether any entry covers it.

## Collect

Read every commit and every PR description that touches the path. Don't sample.

1. Commit list: `git log --format='%H%x09%ad%x09%an%x09%s' --date=short -- <path>`, saved to the raw folder.
2. PR numbers from subjects, in each form the host writes: a squash suffix `(#1234)`, `Merge pull request #1234`, or `See merge request <path>!1234`.
3. Fetch PRs in batches through the host's API, never one call per PR. On GitHub: GraphQL through `gh api graphql`, about 40 aliases per query (`pr1234: pullRequest(number: 1234) { number title body author { login } mergedAt url }`). Save the raw JSON.
4. Lines changed on the path per commit (`git log --numstat -- <path>`), ranked. The top entries are the structural turning points, found faster than by title. When `difft` is installed, drop reformat-only ones first: `DFT_CHECK_ONLY=true GIT_EXTERNAL_DIFF=difft git show --ext-diff --format=%h <sha> -- <path>` prints `No syntactic changes.` for a file whose diff is only formatting. Prose files print `Has changes.` either way.
5. For the biggest PRs with empty descriptions, read the squash commit body (it lists sub-commits) and the diffstat on the path. Label that "why" as inferred.

## Digest

1. Strip PR template boilerplate before reading: checklists, "Type of change", tooling notes. Otherwise the signal drowns.
2. Write a chronological digest, one heading per PR linking to it, cleaned body below.
3. Grep the digest for promises and reversals: `temporary`, `for now`, `will be removed`, `TODO`, `in the future`, `revert`. Check each against the current code before calling it unkept.
4. Group fixes by bug class to find the ones that keep coming back.
5. For "Who to ask": `git shortlog -sn --no-merges -- <path>` for commits per author, and `git log --format='%ad %an' --date=short -- <path>` for each one's first and last commit there.

## Report shape

```markdown
# <Component>: how we got here

Component: [path](link)
<N> commits, <M> PRs, <first month> to <last month>. Raw descriptions in `digest.md`.

---

## TL;DR

<3 to 5 bullets: what shapes today's code>

---

## Timeline

### <year>: <one-word theme>

- **<Mon>.** <what changed, one line>. [#123](link)

---

## Bugs that keep coming back

- **<class>.** <PR links>

---

## Promised, never done

- <quote of the promise>. [#123](link)

---

## Who to ask

- **[handle](link)**: <what they know> (<commit count>)

---

## The story

<5 to 8 short paragraphs, 2 to 3 sentences each, past tense, like a history lesson: why it was born, what reshaped it, what it's still carrying, where it stands today.>

---

## Caveats

- <empty descriptions, inferred rationale, gaps, decision log result>
```

## Writing rules

- Every person named is an inline link too, besides the files, PRs, and docs Readability covers.
- People link to their profile on the code host or to the path's commit history filtered by author. Handles, not guessed full names.
- Readability follows the report-format skill's Readability section.
- Every claim about why something changed cites its PR. One inferred from a diff is labelled per `skills/kit-rules/reference/evidence.md` section 1.
- "The story" is the last content section before Caveats, at most about 300 words, and introduces no fact missing from the sections above.

## Output

- Report at the path `kit scratch-dir history <component-slug>` prints.
- Raw material in a sibling folder named after the report (without `.md`): `prs.json`, `commits.tsv`, `digest.md`.

## Verify before replying

Check by script, not by eye:

1. Every `#<n>` in the report is in the collected PR list and resolves to a PR in this repo.
2. Every linked file or folder exists on the default branch (`git cat-file -e <default>:<path>`).
3. The header counts match `git log -- <path>` and the collected PR list.
4. No TL;DR bullet runs past two lines, and the story is under about 300 words.

## Reply

Per `skills/kit-rules/reference/output.md` section 3.

## Out of scope

- Proposing fixes or refactors.
- Code review of the current state.
- History of code that only imports the path, unless a PR moved code into or out of it.

Arguments: `$ARGUMENTS`
