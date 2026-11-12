# Refactoring and state

- Stopping management without destroying, on OpenTofu 1.12+: a managed resource itself also accepts `lifecycle { destroy = false }`, as a `removed` block does.
- Protection that differs per environment, on OpenTofu 1.12+: have `prevent_destroy` refer to an input variable or another symbol in the module.
- A secret on OpenTofu 1.11+: pass it in an `ephemeral = true` variable, which state never stores and the plan holds only by name. Use it only where ephemeral values are allowed, such as ephemeral resources, ephemeral outputs, and provisioners.
- Change state through configuration that goes through `plan`, never through state CLI commands that don't.
- A renamed or moved resource or module: add a `moved` block, with `from` set to the old address and `to` the new one.
- A shared module: keep its historical `moved` blocks. Deleting one is a breaking change.
- Adopting an existing object: write an `import` block and the resource block, run `plan`, then `apply`, instead of the `tofu import` command.
- An `import` block takes `to` and either `id` or the newer, type-specific `identity`, plus optional `provider` and `for_each`.
- Configuration generation doesn't work with `for_each` on an `import` block.
- Dropping an object from state while leaving the object alone: use a `removed` block, with `from` set to its address and `lifecycle { destroy = false }`.
- A secret in a plain or `sensitive` variable: treat it as readable by anyone who can read state. `sensitive = true` only hides it in output; state holds it in cleartext.
