# Anti-patterns

Severity levels: the report-format skill.

- A rename or module move without a `moved` block: `failure`. Add one with `from` and `to`, and check that the plan shows a move.
- Deleting an old `moved` block from a shared module: `failure`. Keep historical `moved` blocks.
- Deleting a resource block to stop managing an object: `failure`. Use a `removed` block with `lifecycle { destroy = false }`.
- `count` over a list whose items can change: `warning`. Use `for_each` keyed by the item.
- A secret in a plain or `sensitive` variable, treated as protected: `warning`. Use an ephemeral variable where the consumer accepts one (OpenTofu 1.11+), and restrict who can read state.
- A remote module without an exact version: `warning`. Pin an exact version or ref.
- A provisioner where a native mechanism exists: `info`. Use `user_data` or cloud-init, a pre-built image, or a provider resource.
