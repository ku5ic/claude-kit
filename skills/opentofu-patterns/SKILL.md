---
name: opentofu-patterns
description: OpenTofu and Terraform patterns for config-driven refactoring (moved, import, removed), for_each vs count, variables and secrets in state, version pinning and the lock file, fmt/validate in CI, and review-time anti-patterns. Use whenever the project contains `.tf`/`.tfvars` files or a `.terraform.lock.hcl`, OR the user asks about OpenTofu, Terraform, HCL, state, providers, or modules, even if neither tool is mentioned by name.
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

## References

- OpenTofu docs: https://opentofu.org/docs/
- What's new in OpenTofu 1.12: https://opentofu.org/docs/v1.12/intro/whats-new/
- Terraform language docs: https://developer.hashicorp.com/terraform/language

## Version notes

Checked: 2026-11-10 against https://opentofu.org/docs/v1.12/intro/whats-new/ and the GitHub releases of opentofu/opentofu and hashicorp/terraform

- OpenTofu 1.13.0 (2026-09-30, latest 1.13.9 on 2026-11-05; 1.12.12 the same day). No pattern here changed. The unversioned what's-new URL now returns 404, so links point at the 1.12 page. Terraform 1.17.1 (2026-10-28), for reference only.
- OpenTofu 1.12: `lifecycle { destroy = false }`, and `prevent_destroy` may refer to variables.
- OpenTofu 1.11: the `enabled` meta-argument, ephemeral resources, variables and outputs, and write-only attributes.
