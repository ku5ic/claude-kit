---
name: opentofu-patterns
description: OpenTofu and Terraform patterns for moved, import, and removed blocks, for_each, state secrets, and the lock file. Load before any edit to a .tf or .tfvars file, however small, and for HCL questions.
---

# OpenTofu patterns

Default assumption: OpenTofu 1.13. Most rules hold for Terraform too.

- A feature marked "OpenTofu 1.11+" or "1.12+" is an OpenTofu release; check Terraform's own docs before using it in a Terraform project.
- Adapt advice to `required_version` in `versions.tf`, or wherever the `terraform {}` block lives, and to the CLI the project runs, `tofu` or `terraform`.

## Reference files

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                                                                      | Read                                                                     |
| ------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| Renaming, importing, or removing a resource, or touching `lifecycle` or state contents      | [reference/refactoring-and-state.md](reference/refactoring-and-state.md) |
| Writing or changing resources, variables, `for_each` or `count`, versions, or the lock file | [reference/configuration.md](reference/configuration.md)                 |
| Running or wiring `fmt` and `validate`, or running a command the kit's guard may block      | [reference/workflow.md](reference/workflow.md)                           |
| Writing, changing, or reviewing OpenTofu or Terraform code                                  | [reference/anti-patterns.md](reference/anti-patterns.md)                 |
