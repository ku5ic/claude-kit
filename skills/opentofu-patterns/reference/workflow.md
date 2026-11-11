# Workflow

- The project's CI and task runners decide which tofu checks `kit run-checks` runs; `kit run-checks --plan` lists them for a repo.
- `fmt -check -recursive` checks formatting only and needs nothing installed.
- `validate` checks syntax and internal consistency and contacts no backend or provider API.
- Before `validate`: initialize the directory with providers and modules installed. `tofu init -backend=false` does it without touching the backend.
- Run `validate` in pre-commit and CI; `plan` and `apply` validate on their own.
- `guard-bash` blocks `destroy` and `apply -auto-approve` for both `tofu` and `terraform`, even after global options such as `-chdir`.
- Apply a plan only after it's been reviewed.
- When the guard blocks a command, never work around it. Ask the user to run it themselves.
