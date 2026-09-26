---
name: opentofu-patterns
description: OpenTofu and Terraform patterns - config-driven refactoring (moved, import, removed), for_each vs count, variables and secrets in state, version pinning and the lock file, fmt/validate in CI, and review-time anti-patterns. Use whenever the project contains `.tf`/`.tfvars` files or a `.terraform.lock.hcl`, OR the user asks about OpenTofu, Terraform, HCL, state, providers, or modules, even if neither tool is mentioned by name.
---

# OpenTofu patterns

Default assumption: OpenTofu 1.12. Most rules hold for Terraform too, since the language is shared.

- Features marked "OpenTofu 1.11+" or "1.12+" are OpenTofu releases; check Terraform's own docs before using one in a Terraform project.
- Adapt advice to `required_version` in the project's `versions.tf` (or wherever the `terraform {}` block lives) and to which CLI the project runs (`tofu` or `terraform`).

## Reference files

| File                                                                     | Covers                                                                                      |
| ------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------- |
| [reference/refactoring-and-state.md](reference/refactoring-and-state.md) | `moved`, `import`, `removed` blocks, `lifecycle`, what state holds                          |
| [reference/configuration.md](reference/configuration.md)                 | `for_each` vs `count`, `enabled`, variables, pinning and the lock file, provisioners         |
| [reference/workflow.md](reference/workflow.md)                           | `fmt` and `validate` in checks, and what the kit's guard blocks                             |
| [reference/anti-patterns.md](reference/anti-patterns.md)                 | Seven review-time anti-patterns with severity calls                                         |

## References

- OpenTofu docs: https://opentofu.org/docs/
- What's new in OpenTofu: https://opentofu.org/docs/intro/whats-new/
- Terraform language docs: https://developer.hashicorp.com/terraform/language

## Version notes

Checked: 2026-09-26 against https://opentofu.org/docs and the GitHub releases of opentofu/opentofu and hashicorp/terraform

- OpenTofu 1.12.6 (2026-08-19); 1.12: `lifecycle { destroy = false }`, and `prevent_destroy` may refer to variables. 1.13.0-rc1 (2026-09-17) is a release candidate, not covered.
- OpenTofu 1.11: the `enabled` meta-argument, ephemeral resources, variables and outputs, and write-only attributes.
- Terraform 1.16.4 (2026-09-23), for reference only.
