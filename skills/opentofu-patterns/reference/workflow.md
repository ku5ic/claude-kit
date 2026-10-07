# Workflow

## Checks

kit.yml's `toolchain_checks` owns which tofu checks `kit run-checks` runs; `kit run-checks --plan` shows them for a repo. What the two commands need:

- `fmt -check -recursive`: formatting only, needs nothing installed.
- `validate`: syntax and internal consistency. It contacts no backend or provider API, but it needs an initialized directory with providers and modules installed. To initialize without touching the backend, run `tofu init -backend=false`.

`plan` and `apply` validate automatically; `validate` is for pre-commit and CI. Source: https://opentofu.org/docs/cli/commands/validate

## What the guard blocks

`guard-bash` blocks `destroy` and `apply -auto-approve` for both `tofu` and `terraform`, with global options like `-chdir` in front. A plan has to be reviewed before it's applied. Never work around the block; ask the user to run it themselves.
