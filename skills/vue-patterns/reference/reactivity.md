# Reactivity

- Declaring reactive state: `ref()`, the primary API per the Vue docs. It takes primitives, objects, and arrays, and unwraps in templates and inside `reactive()` containers.
- A `ref` in script: read and write `.value`. Top-level refs in `<script setup>` reach the template, where `.value` is implicit.
- `reactive()` takes objects only, and `reactive("hello")` returns the string unchanged. Use `ref()` for a primitive.
- A `reactive()` object: mutate it in place, never swap it with `state = reactive(otherObj)`. Every consumer holding the original reference stops updating.
- Destructuring a `reactive()` object: `let { count } = state` copies a plain value that no longer updates `state`. Use `const { count } = toRefs(state)` for one ref per property.
- A large object replaced wholesale on each update, such as a fetched server response: `shallowRef` or `shallowReactive`, which track only the top level.
- A third-party class instance whose internals Vue shouldn't mutate: `shallowRef` or `shallowReactive`.
- After mutating a `shallowRef` in place, call `triggerRef(ref)` to trigger updates.
- Use `reactive` over `ref` only for one object that lives as long as the component, has many properties, and is mutated in place, not replaced. Even then, use `ref` when a consumer destructures it or a top-level property is a primitive.
