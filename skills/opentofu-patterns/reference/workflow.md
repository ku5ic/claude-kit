# Workflow

## Checks

`run-checks.sh` runs two checks where a `.terraform.lock.hcl` is, with `tofu`, or `terraform` when `tofu` isn't installed:

- `fmt -check -recursive`: formatting only, needs nothing installed.
- `validate`: syntax and internal consistency. It contacts no backend or provider API, but it needs an initialized directory with providers and modules installed, so the check reports SKIP until `.terraform/` exists. To initialize without touching the backend, run `tofu init -backend=false`.

`plan` and `apply` validate automatically; `validate` is for pre-commit and CI. Source: https://opentofu.org/docs/cli/commands/validate

## What the guard blocks

`guard-bash.sh` blocks `destroy` and `apply -auto-approve` for both `tofu` and `terraform`, with global options like `-chdir` in front. A plan has to be reviewed before it's applied. Never work around the block; ask the user to run it themselves.
