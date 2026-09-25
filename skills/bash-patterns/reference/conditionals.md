# Conditionals and tests

- `[[ ... ]]` over `[ ... ]` in bash scripts. `[[` does NOT word-split or pathname-expand its operands, so `[[ $var = pattern ]]` is safe even unquoted on the left side. `[ ]` is the POSIX `test` builtin and does both, so unquoted operands are bugs.
- Inside `[[ ]]`: `==` and `=` are equivalent. `==` reads better. The right side of `==`/`!=` is treated as a glob pattern unless it is quoted: `[[ "$x" == *.log ]]` matches; `[[ "$x" == "*.log" ]]` is a literal-string compare.
- Numeric comparison inside `[[ ]]`: `-eq`, `-ne`, `-lt`, `-le`, `-gt`, `-ge`. Do NOT use `==` for numbers; that is a string compare and `[[ 01 == 1 ]]` is false.
- `(( expression ))` for arithmetic. Returns 0 if non-zero, 1 if zero, which inverts the natural reading; remember `(( count == 0 ))` returns 1 (false in shell, true in C).
- `case "$x" in pattern) cmd ;; esac` for multi-branch dispatch. Patterns use the same glob syntax as pathname expansion.
- `&&` and `||` for terse one-liners. Avoid for multi-statement flow; reach for `if/then/else`.

## References

- GNU Bash manual (Conditional Constructs): https://www.gnu.org/software/bash/manual/bash.html#Conditional-Constructs
- BashGuide on tests: https://mywiki.wooledge.org/BashGuide/TestsAndConditionals
