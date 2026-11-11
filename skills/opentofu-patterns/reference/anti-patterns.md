# Anti-patterns

Severity levels: the report-format skill.

## A rename or module move without a `moved` block

`failure`. The plan destroys the object at the old address and creates a new one. For a database or a bucket, that's data loss. Add a `moved` block with `from` and `to`, and check the plan shows a move.

## Deleting an old `moved` block from a shared module

`failure`. Callers still on the old address plan to delete the object instead of moving it. Keep historical `moved` blocks.

## Deleting a resource block to stop managing an object

`failure`. Removing the block destroys the object. Use a `removed` block with `lifecycle { destroy = false }`.

## `count` over a list whose items can change

`warning`. Removing a middle item shifts every later index, so those instances update or get replaced. Use `for_each` keyed by the item.

## A secret in a plain or `sensitive` variable, treated as protected

`warning`. `sensitive` only hides output; the value is in state in cleartext. Use an ephemeral variable where the consumer accepts one (OpenTofu 1.11+), and restrict who can read state.

## Remote module without an exact version

`warning`. The lock file doesn't lock modules, so every `init` can pick a newer version inside the constraint. Pin an exact version or ref.

## A provisioner where a native mechanism exists

`info`. Provisioners are a last resort. Use `user_data` or cloud-init, a pre-built image, or a provider resource.
