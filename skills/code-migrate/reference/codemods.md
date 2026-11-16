# Codemods

- A codemod the library's vendor names in its upgrade guide beats a hand-written one.
- Preview first. `sg run -p P -r R` prints the rewrite as a diff and changes nothing, while `-U` would apply it. jscodeshift and `@next/codemod` preview with `--dry --print`.
- A codemod can leave spots it couldn't fix. Next's codemods mark them with `@next-codemod-error` comments, which fail the build until resolved. Count them as hand work in the batch.
- A codemod run on a wrong path can report success with nothing changed. Check the count of files it touched.
- `cargo fix --edition` doesn't update doctests, and macros or generated code may need hand edits.
- `gofmt -r 'pattern -> replacement'` rewrites Go expressions, and `-l` lists the files it would change.
- The dependency bump, the codemod run, and the formatting go in separate commits, so each diff stays reviewable.
