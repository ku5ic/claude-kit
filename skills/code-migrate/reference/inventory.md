# Inventory

- `sg run -p '<pattern>' -l <lang> <path>` searches by syntax tree. A pattern has to be code the language's parser accepts.
- `$A` matches one node, so `console.log($A)` misses `console.log(a, b)`. `$$$A` matches a run of nodes.
- A pattern that parses wrong gets more surrounding code, or a rule file with a `kind`, never a looser pattern.
- `--json=stream` prints one match per line, with `file`, `range`, `lines`, `text`, and the captures under `metaVariables`.
- `sg` knows a fixed set of languages. Code in any other gets `rg` only, and the report says so.
- `rg -w <name>` finds what no pattern parses (config, strings, docs, generated code). Those hits are listed apart from the syntax-tree ones.
