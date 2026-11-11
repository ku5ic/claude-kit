---
name: opentofu-patterns
description: OpenTofu and Terraform patterns for moved, import, and removed blocks, for_each, state secrets, and the lock file. Use for .tf or .tfvars files, .terraform.lock.hcl, or HCL questions.
---

# OpenTofu patterns

Default assumption: OpenTofu 1.13. Most rules hold for Terraform too, since the language is shared.

- Features marked "OpenTofu 1.11+" or "1.12+" are OpenTofu releases; check Terraform's own docs before using one in a Terraform project.
- Adapt advice to `required_version` in the project's `versions.tf` (or wherever the `terraform {}` block lives) and to which CLI the project runs (`tofu` or `terraform`).

## Reference files

| File                                                                     | Covers                                                                               |
| ------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ |
| [reference/refactoring-and-state.md](reference/refactoring-and-state.md) | `moved`, `import`, `removed` blocks, `lifecycle`, what state holds                   |
| [reference/configuration.md](reference/configuration.md)                 | `for_each` vs `count`, `enabled`, variables, pinning and the lock file, provisioners |
| [reference/workflow.md](reference/workflow.md)                           | `fmt` and `validate` in checks, and what the kit's guard blocks                      |
| [reference/anti-patterns.md](reference/anti-patterns.md)                 | Seven review-time anti-patterns with severity calls                                  |
