---
name: meta-conventions
description: Write down the conventions a repo follows, as path-scoped rules with a citation for each
argument-hint: "<optional: areas to cover, comma-separated>"
disable-model-invocation: true
---

# /meta-conventions

Find the conventions this repo actually follows and write them down once, as path-scoped rules with a citation behind every claim, so later sessions read them instead of rediscovering them.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                      | Read                                                 |
| --------------------------- | ---------------------------------------------------- |
| Writing a rule file, step 6 | [reference/rules-files.md](reference/rules-files.md) |

## Procedure

1. Scope. The arguments are: $ARGUMENTS. Resolve the project root with `kit project-root`. Use the `<repo-context>` block for the stack.
2. Ask via AskUserQuestion, in one call:
   - Which areas to cover (multi-select): state, data fetching, styling, testing, errors, naming, file layout. Areas named in the arguments are pre-answered; skip this question when they cover everything.
   - Whether to write the rules under `<root>/.claude/rules/conventions/` (one file per area) or only report them. These files are meant to be committed, so writing needs a yes.
3. Investigate each chosen area with the `investigate` skill: "What convention does this repo follow for <area>? Cite every instance as `file:line`." For independent areas, fan out as the skill allows.
4. Review comments, when the repo is on a code host (`skills/kit-rules/reference/tooling.md` section 4): the review comments of the last 90 days, by the code-host-patterns skill's "Review comments across the repo" call. Group the ones that make the same point. A point made in at least two comments on at least two PRs, about a chosen area, is a candidate convention: confirm it in the code like any other, and cite the comments next to its code instances. For each, name the cheapest enforcement in the report (a lint rule, an `sg` rule, or this rule file's line), per `skills/kit-rules/reference/change.md` section 7.
5. Keep a convention only when it has **at least two** cited `file:line` instances that you've read yourself, and no cited counter-examples that outnumber them. A single instance is an example, not a convention. An area left with nothing is recorded as `no convention found` in the report. It gets no rule file, since an empty rule costs context and says nothing.
6. Write one file per area with at least one convention: `<root>/.claude/rules/conventions/<area>.md`, per `skills/kit-rules/reference/output.md` section 3:

   ```markdown
   ---
   paths:
     - "<glob covering where this convention applies>"
   ---

   # Conventions: <area>

   Written by /meta-conventions on <YYYY-MM-DD>. `/audit-doc-drift` re-checks the citations.

   - <Convention, stated as an instruction>. Seen at `<path>:<line>`, `<path>:<line>`[; asked for in review: <comment link>, <comment link>].
   ```

   - `paths` is the narrowest set of globs where the convention holds, such as `"src/**/*.{ts,tsx}"` for a component convention. Leave `paths` out only for a convention that truly applies to every file (naming, sometimes). When each loads: [reference/rules-files.md](reference/rules-files.md).
   - Every bullet ends with its citations. A bullet without one gets cut.
   - Record what the code does, not what it should do. A convention you'd argue against still goes in as found; raise the disagreement in the report.

7. Report: the files written, each with its convention count; the areas marked `no convention found`; any conventions with notable counter-examples, which are the candidates for a cleanup; and each review-comment convention with its cheapest enforcement.

## Rules

- A convention is only as good as its citations. Read each cited line before writing it down.
- Never overwrite an existing file under `.claude/rules/conventions/` without showing the diff and asking.
- Commit nothing. The user reviews the rules first.
