# Refactoring and state

Change state through configuration that goes through `plan`, not through state CLI commands that don't.

## `moved`: renames and restructuring

Without a `moved` block, renaming a resource or module plans a destroy at the old address and a create at the new one. With one, the existing object is treated as belonging to the new address:

```hcl
moved {
  from = aws_instance.a
  to   = aws_instance.b
}
```

Keep historical `moved` blocks in shared modules. Deleting one is a breaking change: a caller still on the old address plans to delete the object instead of moving it. Source: https://opentofu.org/docs/language/modules/develop/refactoring/

## `import`: adopting existing objects

An `import` block takes `to` (the address) and `id` or `identity` (`identity` is the newer, type-specific form), plus optional `provider` and `for_each`. Unlike the `tofu import` command it's previewed in `plan` and works in CI: write the import block and the resource block, plan, then apply. Configuration generation doesn't work together with `for_each` on an import block. Source: https://opentofu.org/docs/language/import/

## `removed`: stop managing without destroying

Deleting a resource block destroys the object. To drop it from state and leave the object alone:

```hcl
removed {
  from = aws_instance.web
  lifecycle {
    destroy = false
  }
}
```

Source: https://opentofu.org/docs/language/resources/syntax/

OpenTofu 1.12+ also accepts `lifecycle { destroy = false }` on a managed resource itself. Source: https://opentofu.org/docs/intro/whats-new/

## `lifecycle` guards

In OpenTofu 1.12+, `prevent_destroy` may refer to other symbols in the module, such as input variables, so protection can differ per environment. Source: https://opentofu.org/docs/intro/whats-new/

## What state holds

- `sensitive = true` only hides a value in output. OpenTofu still records it in state in cleartext, so anyone who can read state reads it. Source: https://opentofu.org/docs/language/values/variables/
- An `ephemeral = true` variable (OpenTofu 1.11+) is never stored in state; only its name goes into the plan. It can only be used where ephemeral values are allowed, such as ephemeral resources, ephemeral outputs, and provisioners. Source: https://opentofu.org/docs/language/values/variables/
- So a secret passed through a plain or `sensitive` variable is a state-access problem, not only a display problem.
