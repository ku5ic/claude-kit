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

## reference/anti-patterns.md

### Claims

- A rename or module move without a `moved` block, `failure`. The plan destroys the object at the old addres...: https://opentofu.org/docs/language/modules/develop/refactoring/
- Deleting an old `moved` block from a shared module, `failure`. Callers still on the old address plan to delet...: https://opentofu.org/docs/language/modules/develop/refactoring/
- Deleting a resource block to stop managing an object, `failure`. Removing the block destroys the object. Use a...: https://opentofu.org/docs/language/resources/syntax/
- `count` over a list whose items can change, `warning`. Removing a middle item shifts every later inde...: https://opentofu.org/docs/language/meta-arguments/count/
- A secret in a plain or `sensitive` variable, treated as protected, `warning`. `sensitive` only hides output; the value is in...: https://opentofu.org/docs/language/values/variables/
- Remote module without an exact version, `warning`. The lock file doesn't lock modules, so every `...: https://opentofu.org/docs/language/files/dependency-lock/
- A provisioner where a native mechanism exists, `info`. Provisioners are a last resort. Use `user_data` o...: https://opentofu.org/docs/language/resources/provisioners/syntax/

## reference/configuration.md

### Claims

- `for_each` over `count` for collections, `count` identifies instances by index. Remove an element...: https://opentofu.org/docs/language/meta-arguments/count/
- `enabled` for zero-or-one (OpenTofu 1.11+), For a resource or module that either exists or doesn't, `...: https://opentofu.org/docs/language/meta-arguments/enabled/
- Variables: https://opentofu.org/docs/language/values/variables/
- Pinning and the lock file: https://opentofu.org/docs/language/files/dependency-lock/
- Provisioners are a last resort, Provisioners add complexity and uncertainty, and state ca...: https://opentofu.org/docs/language/resources/provisioners/syntax/

## reference/refactoring-and-state.md

### Claims

- `moved`: renames and restructuring, Keep historical `moved` blocks in shared modules. Deletin...: https://opentofu.org/docs/language/modules/develop/refactoring/
- `import`: adopting existing objects, An `import` block takes `to` (the address) and `id` or `i...: https://opentofu.org/docs/language/import/
- `removed`: stop managing without destroying: https://opentofu.org/docs/language/resources/syntax/
- `removed`: stop managing without destroying, OpenTofu 1.12+ also accepts `lifecycle { destroy = false...: https://opentofu.org/docs/v1.12/intro/whats-new/
- `lifecycle` guards, In OpenTofu 1.12+, `prevent_destroy` may refer to other s...: https://opentofu.org/docs/v1.12/intro/whats-new/
- What state holds, `sensitive = true` only hides a value in output. OpenTofu...: https://opentofu.org/docs/language/values/variables/
- What state holds, An `ephemeral = true` variable (OpenTofu 1.11+) is never...: https://opentofu.org/docs/language/values/variables/

## reference/workflow.md

### Claims

- Checks, `plan` and `apply` validate automatically; `validate` is...: https://opentofu.org/docs/cli/commands/validate
