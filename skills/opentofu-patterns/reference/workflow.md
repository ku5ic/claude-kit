# Workflow

- The project's CI and task runners decide which tofu checks `kit run-checks` runs; `kit run-checks --plan` lists them for a repo.
- `fmt -check -recursive` checks formatting only and needs nothing installed.
- `validate` checks syntax and internal consistency and contacts no backend or provider API.
- Before `validate`: initialize the directory with providers and modules installed. `tofu init -backend=false` does it without touching the backend.
- Run `validate` in pre-commit and CI; `plan` and `apply` validate on their own.
- Apply a plan only after it's been reviewed.
- A command the bash guard blocks goes to the user, per `skills/kit-rules/reference/workflow.md` section 4.
