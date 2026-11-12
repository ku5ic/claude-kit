# Conditionals and tests

- A test in a bash script: `[[ ]]`, not `[ ]`. `[[` doesn't word-split or glob its operands, so an unquoted left side is safe.
- Inside `[[ ]]`, `==` and `=` are the same. Write `==`.
- The right side of `==` or `!=` is a glob unless quoted: `[[ "$x" == *.log ]]` matches the pattern, `[[ "$x" == "*.log" ]]` compares the literal string.
- Comparing numbers: `-eq`, `-ne`, `-lt`, `-le`, `-gt`, `-ge`. Never `==`, a string compare: `[[ 01 == 1 ]]` is false.
- Arithmetic: `(( expression ))`. It returns 0 when the expression is non-zero and 1 when it's zero.
- Multi-branch dispatch: `case "$x" in pattern) cmd ;; esac`. Its patterns use pathname-expansion glob syntax.
- `&&` and `||`: terse one-liners only. Multi-statement flow gets `if/then/else`.
