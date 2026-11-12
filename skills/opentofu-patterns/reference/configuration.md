# Configuration

- A resource or module that either exists or doesn't, on OpenTofu 1.11+: set `enabled` inside `lifecycle`, not `count = var.x ? 1 : 0`.
- A reference to a resource that has `enabled`: guard it against `null` with `try(aws_instance.example.id, "not-created")` or a `!= null` check.
- N identical things: use `count`. One per item of a collection: use `for_each`, keyed by the item's value.
- An input rule: put it in a `validation` block, a `condition` with an `error_message`, not in a later failure inside a resource.
- A variable the module can't handle as `null`: set `nullable = false`. The default is `true`.
- Commit `.terraform.lock.hcl`.
- The lock file tracks providers only, and OpenTofu picks the newest remote module version that meets the constraint. Pin shared modules to an exact version or ref.
- Before a `local-exec` or `remote-exec` provisioner, use instance creation data (`user_data`, `custom_data`, `metadata`), cloud-init, an image pre-built with Packer and config management, or the provider's own resources.
