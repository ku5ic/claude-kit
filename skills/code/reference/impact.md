# /code impact

What breaks if a file or symbol changes: its consumers, the tests covering each, and how often each moves. Arguments: `<file path> [symbol]`. With no argument, ask for the file before anything else.

## Consumers

1. Resolve the argument to a tracked file. A loose name gets searched with `fd`, and the substitution goes in the report header.
2. JS, TS, or Python: `kit blast-radius <file> [symbol]`. Each row is `file:line`, `source` or `test`. An `unresolvable imports present` line goes in Caveats.
3. Any other language:

```bash
kit code-graph imports <root> > <raw>/imports.tsv
kit code-graph deps <root> <raw>/imports.tsv <raw>
```

- A consumer is a row of `<raw>/imports-resolved.tsv` whose second column is the target's module: its path without the extension, or its directory for Go.
- A Go import names the package, not the file. Keep a consumer only when it uses an exported name the target file defines.
- Go files in the target's own package use it without an import. List them too, except `_test.go` files, which count as tests.
- With a symbol, keep only consumers where `rg -w <symbol>` matches.

4. A consumer that is itself a shared module (more than 5 consumers of its own) gets one more hop, listed under it. Never more than one.

## Tests and churn

For each source consumer:

- **Tests**: the test files that import it, by the same command as step 2 or 3 run on the consumer. For Go, also the `_test.go` files in its directory. None is a finding.
- **Churn**: `git log --since=6.months --format=%h -- <consumer> | wc -l`.
- For the target itself: `git log --since=6.months --format='%h %ad %s' --date=short -- <target> | head -15`, read for why it keeps changing.

Then read the target's exported surface and each consumer's call site. A consumer that relies on behavior beyond the signature (an error string, ordering, a default, a side effect) is the one that breaks without a type error.

## Report shape

Write to the path `kit scratch-dir impact <target-slug>` prints, raw files in a sibling folder named after it without `.md`.

```markdown
# Impact: <target> [<symbol>]

<N> consumers (<source> source, <test> test), <M> after one hop. Window: 6 months.

## Summary

<Two or three sentences: how far a change reaches, where it's least covered, and the one consumer to check first.>

## Consumers

| Consumer | Uses | Tests | Commits |
| -------- | ---- | ----- | ------- |

## Fragile call sites

- <file:line>: <what it relies on beyond the signature, and what breaks>

## Caveats

- <unresolvable imports, a language with no scanner, reflection or string dispatch>
```

`Uses` is the symbol or call the consumer touches. A consumer with no tests and the most commits sorts first. Fragile call sites and Caveats are left out when empty.

## Reply

The report path, the consumer count, and the first consumer to check.

## Out of scope

- Changing the code, or proposing a refactor.
- Runtime consumers outside the repo (other services, published package users), named in Caveats only when the target is a public API.
