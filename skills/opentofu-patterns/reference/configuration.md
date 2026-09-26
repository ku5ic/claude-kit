# Configuration

## `for_each` over `count` for collections

`count` identifies instances by index. Remove an element from the middle of the list and every instance after it gets a new index, so its arguments change and it's updated or replaced. `for_each` keys instances by the element's value, so removal touches only that instance. Use `count` for "N identical things", `for_each` for "one per item". Source: https://opentofu.org/docs/language/meta-arguments/count/

## `enabled` for zero-or-one (OpenTofu 1.11+)

For a resource or module that either exists or doesn't, `enabled` goes inside `lifecycle`, replacing the `count = var.x ? 1 : 0` idiom. A disabled resource evaluates to `null`, so references need a guard: `try(aws_instance.example.id, "not-created")` or a `!= null` check. Source: https://opentofu.org/docs/language/meta-arguments/enabled/

## Variables

- `validation` blocks pair a `condition` with an `error_message`; put input rules there instead of failing later in a resource.
- `nullable` defaults to `true`. Set `nullable = false` when the module can't handle `null`, so the variable never is.

Source: https://opentofu.org/docs/language/values/variables/

## Pinning and the lock file

- Commit `.terraform.lock.hcl`, so dependency changes go through code review like config changes.
- The lock file tracks providers only. OpenTofu always picks the newest remote module version that meets the constraint, so pin shared modules with exact versions or refs.

Source: https://opentofu.org/docs/language/files/dependency-lock/

## Provisioners are a last resort

Before a `local-exec` or `remote-exec` provisioner, use:

- instance creation data: `user_data`, `custom_data`, `metadata`
- cloud-init
- images pre-built with Packer and config management
- the provider's own resources

Provisioners add complexity and uncertainty, and state can't track what they did. Source: https://opentofu.org/docs/language/resources/provisioners/syntax/
