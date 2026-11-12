# opentofu-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- OpenTofu docs: https://opentofu.org/docs/
- What's new in OpenTofu 1.12: https://opentofu.org/docs/v1.12/intro/whats-new/
- Terraform language docs: https://developer.hashicorp.com/terraform/language

### Version notes

Checked: 2026-11-10 against https://opentofu.org/docs/v1.12/intro/whats-new/ and the GitHub releases of opentofu/opentofu and hashicorp/terraform

- OpenTofu 1.13.0 (2026-09-30, latest 1.13.9 on 2026-11-05; 1.12.12 the same day). No pattern here changed. The unversioned what's-new URL now returns 404, so links point at the 1.12 page. Terraform 1.17.1 (2026-10-28), for reference only.
- OpenTofu 1.12: `lifecycle { destroy = false }`, and `prevent_destroy` may refer to variables.
- OpenTofu 1.11: the `enabled` meta-argument, ephemeral resources, variables and outputs, and write-only attributes.

### Left to linters

Checked: 2026-11-11. No default check in the kit's set (go vet and golangci-lint defaults, ruff defaults, ESLint `js.configs.recommended`, shellcheck with no flags, `tsc --strict`) reads HCL, so no rule here was dropped.

## reference/refactoring-and-state.md

### Claims

- `lifecycle { destroy = false }` on a managed resource 1.12+, `prevent_destroy` referring to variables 1.12+: https://opentofu.org/docs/v1.12/intro/whats-new/
- Ephemeral variables 1.11+, `sensitive` only hiding output while state holds the value: https://opentofu.org/docs/language/values/variables/
- `moved` blocks, keeping historical ones in shared modules: https://opentofu.org/docs/language/modules/develop/refactoring/
- `import` block arguments, plan-then-apply, no configuration generation with `for_each`: https://opentofu.org/docs/language/import/
- `removed` block with `lifecycle { destroy = false }`: https://opentofu.org/docs/language/resources/syntax/

## reference/configuration.md

### Claims

- `enabled` in `lifecycle` 1.11+, `null` when disabled: https://opentofu.org/docs/language/meta-arguments/enabled/
- `count` for identical things, `for_each` keyed by item: https://opentofu.org/docs/language/meta-arguments/count/
- `validation` blocks, `nullable`: https://opentofu.org/docs/language/values/variables/
- Committing the lock file, providers only, pinning remote modules: https://opentofu.org/docs/language/files/dependency-lock/
- Native mechanisms before provisioners: https://opentofu.org/docs/language/resources/provisioners/syntax/

## reference/workflow.md

### Claims

- `plan` and `apply` validate on their own; `validate` for pre-commit and CI: https://opentofu.org/docs/cli/commands/validate

## reference/anti-patterns.md

### Claims

- A rename without `moved`, deleting an old `moved` block: https://opentofu.org/docs/language/modules/develop/refactoring/
- Deleting a resource block instead of a `removed` block: https://opentofu.org/docs/language/resources/syntax/
- `count` over a changing list: https://opentofu.org/docs/language/meta-arguments/count/
- A secret in a plain or `sensitive` variable: https://opentofu.org/docs/language/values/variables/
- A remote module without an exact version: https://opentofu.org/docs/language/files/dependency-lock/
- A provisioner where a native mechanism exists: https://opentofu.org/docs/language/resources/provisioners/syntax/
