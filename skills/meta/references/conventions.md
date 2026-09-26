# /meta conventions

Find the conventions this repo actually follows and write them down once, as path-scoped rules with a citation behind every claim, so later sessions read them instead of rediscovering them. Arguments: `<optional: areas to cover, comma-separated>`.

## Procedure

1. Scope. The arguments are: $ARGUMENTS. Resolve the project root with `!`project-root.sh``. Use the `<repo-context>` block for the stack.
2. Ask via AskUserQuestion, in one call:
   - Which areas to cover (multi-select): state, data fetching, styling, testing, errors, naming, file layout. Areas named in the arguments are pre-answered; skip this question when they cover everything.
   - Whether to write the rules under `<root>/.claude/rules/conventions/` (one file per area) or only report them. These files are meant to be committed, so writing needs a yes.
3. Investigate each chosen area with the `investigate` skill: "What convention does this repo follow for <area>? Cite every instance as `file:line`." For independent areas, fan out as the skill allows.
4. Keep a convention only when it has **at least two** cited `file:line` instances that you've read yourself, and no cited counter-examples that outnumber them. A single instance is an example, not a convention. An area left with nothing is recorded as `no convention found` in the report. It gets no rule file, since an empty rule costs context and says nothing.
5. Write one file per area with at least one convention: `<root>/.claude/rules/conventions/<area>.md`. Use Write, never a shell redirect:

   ```markdown
   ---
   paths:
     - "<glob covering where this convention applies>"
   ---

   # Conventions: <area>

   Written by /meta conventions on <YYYY-MM-DD>. `/audit doc-drift` re-checks the citations.

   - <Convention, stated as an instruction>. Seen at `<path>:<line>`, `<path>:<line>`.
   ```

   - `paths` is the narrowest set of globs where the convention holds, such as `"src/**/*.{ts,tsx}"` for a component convention. Leave `paths` out only for a convention that truly applies to every file (naming, sometimes). Claude Code loads a path-scoped rule when it reads a matching file, and loads a rule without `paths` in every session.
   - Every bullet ends with its citations. A bullet without one gets cut.
   - Record what the code does, not what it should do. A convention you'd argue against still goes in as found; raise the disagreement in the report.
6. Report: the files written, each with its convention count; the areas marked `no convention found`; and any conventions with notable counter-examples, which are the candidates for a cleanup.

## Rules

- A convention is only as good as its citations. Read each cited line before writing it down.
- Never overwrite an existing file under `.claude/rules/conventions/` without showing the diff and asking.
- Commit nothing. The user reviews the rules first.
