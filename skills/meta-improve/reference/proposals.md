# Proposals

## The bar

A proposal needs all four. Missing one, it's rejected, naming the one.

- Verbatim quotes from at least two sessions, each with its `path:line`, each read this run. Two quotes from one session, or from a session and its resume, count once.
- The file it changes, read this run, and the exact text before and after.
- Its word cost, measured.
- A follow-up date and what to look for.

A change rejected in an earlier round needs a quote newer than that round.

## Word cost

- Run `wc -w` on the file as it is, and on a copy in scratch with the change made. Report both, and the difference.
- A skill's `description` and an always-loaded file cost every session; a reference file costs the sessions that read it. Say which.
- A remove or refine that costs words is reported as costing them.

## Follow-up

- The date is two weeks after the report's.
- What to look for is a behavior visible in a transcript, with the filter that finds it (a rule's name, a tool error, a command, a phrase) and the count and window it was drawn from. The next round uses the same filter.
- Say what kind of session could show it. When the window since then holds none, the follow-up establishes nothing.

## Where it's edited

- A kit rule, skill, or hook: the kit's source checkout, found as `/meta-skill` step 4 finds it. With no checkout, name the file and the `kit.yml` overlay key that does the same on this machine, if one does.
- The user's own CLAUDE.md, rules, and skills: the real path behind a symlink.
- A project's CLAUDE.md or `.claude/rules/`: that project's.

## Shape

````markdown
### <Remove, Refine, Relocate, or Add>: <the change in a few words>

<One sentence: the behavior, and how many sessions show it.>

- **Class:** <misguiding, weak, unreachable, or uncovered>, <the fact that decides it, with its `path:line`>
- **Evidence:** "<quote>" `<path:line>`, "<quote>" `<path:line>`
- **Change:** `<file:line>`

  ```diff
  - <before>
  + <after>
  ```

- **Words:** <before> to <after> in `<file>`, paid by <every session, or the sessions that read it>
- **Follow-up:** <YYYY-MM-DD>: <the behavior>, found by <filter>, <count> in <window>. Nothing to conclude when <no session of this kind ran>
````
