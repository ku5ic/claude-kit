---
name: code-impact
description: What breaks if a file or symbol changes: its consumers, their tests, and their churn
argument-hint: "<file path> [symbol]"
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

# /code-impact

What breaks if a file or symbol changes: its consumers, the tests covering each, and how often each moves. With no argument, ask for the file before anything else.

## Consumers

1. Resolve the argument to a tracked file. A loose name gets searched with `fd`, and the substitution goes in the report header.
2. Build the import graph:

```bash
kit code-graph imports <root> > <raw>/imports.tsv
kit code-graph deps <root> <raw>/imports.tsv <raw>
```

- For a language `imports` doesn't cover, append rows as `skills/code-visual/SKILL.md` Collect step 4 does. Without ast-grep, use its fallback, and say so in the report.
- A consumer is a row of `<raw>/imports-resolved.tsv` whose second column is the target or the unit holding it (its module, package, or directory).
- An import that names a unit wider than the file counts only when the consumer uses a name the target defines.
- Files that share the target's unit can use it without an import. List the ones that use a name it defines.
- With a symbol, keep only consumers where `rg -w <symbol>` matches.
- A file the project's test naming matches is a test, not a consumer.

3. Imports miss reflection, string dispatch, config that names the target, and generated code. `rg` for the target's file name and exported names outside the graph's hits, and list what turns up under Caveats.

4. A consumer that is itself a shared module (more than 5 consumers of its own) gets one more hop, listed under it. Never more than one.

## Tests and churn

For each source consumer:

- **Tests**: the test files that reach it, by the same lookup as step 2 run on the consumer. None is a finding.
- **Churn**: `git log --since=6.months --format=%h -- <consumer> | wc -l`.
- For the target itself: `git log --since=6.months --format='%h %ad %s' --date=short -- <target> | head -15`, read for why it keeps changing.

Then read the target's exported surface and each consumer's call site. A consumer that relies on behavior beyond the signature (an error string, ordering, a default, a side effect) is the one that breaks without a type error.

## Report shape

Write to the path `kit scratch-dir impact <target-slug>` prints, raw files in a sibling folder named after it without `.md`.

```markdown
# Impact: <target> [<symbol>]

<N> consumers and <T> tests, <M> after one hop. Window: 6 months.

## TL;DR

- <how far a change reaches>
- <the one consumer to check first, and why>
- <where it's least covered>

---

## Consumers

| Consumer | Uses | Tests | Commits |
| -------- | ---- | ----- | ------- |

---

## Fragile call sites

- <file:line>: <what it relies on beyond the signature, and what breaks>

---

## Caveats

- <unresolvable imports, a language with no scanner, reflection or string dispatch>
```

`Uses` is the symbol or call the consumer touches. A consumer with no tests and the most commits sorts first. Fragile call sites and Caveats are left out when empty.

## Reply

The report path, the consumer count, and the first consumer to check.

## Out of scope

- Changing the code, or proposing a refactor.
- Runtime consumers outside the repo (other services, published package users), named in Caveats only when the target is a public API.

Arguments: `$ARGUMENTS`
