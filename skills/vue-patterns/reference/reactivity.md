# Reactivity

## `ref` is the primary API

The Vue docs are explicit: "we recommend using `ref()` as the primary API for declaring reactive state." `ref(value)` wraps any value in a tracked container with a `.value` property. It works for primitives, objects, and arrays, survives destructuring (you destructure `.value`, not the ref itself), and unwraps automatically in templates and `reactive()` containers.

```ts
import { ref, computed } from "vue";

const count = ref(0);
const doubled = computed(() => count.value * 2);

count.value++; // triggers updates
```

In `<script setup>`, top-level refs are auto-exposed to the template, where `.value` is implicit:

```vue
<template>
  <button @click="count++">{{ count }}</button>
</template>
```

## `reactive` and its three traps

`reactive(obj)` wraps an object in a Proxy. The state is the object itself; no `.value` indirection. The trade-off is three documented limitations:

- **Primitives are not allowed.** `reactive("hello")` returns the string unchanged.
- **Cannot replace the whole object.** `state = reactive({...})` then later `state = reactive(otherObj)` breaks every consumer that captured the original reference.
- **Destructuring loses reactivity.** Vue tracks property access on the proxy; pulling a property out into a local binding extracts the value and severs the link.

```ts
const state = reactive({ count: 0 });
let { count } = state; // count is now a plain number
count++; // does NOT update state.count
```

To destructure safely, use `toRefs(state)`:

```ts
import { toRefs } from "vue";
const { count } = toRefs(state); // each property is a ref
```

## `shallowRef` and `shallowReactive`

`ref` and `reactive` recurse: nested objects are also made reactive. `shallowRef` and `shallowReactive` track only the top level. Use them when:

- The state is a large object replaced wholesale on each update (e.g. fetched server response). Deep tracking is wasted work.
- The object is a third-party class instance whose internals should not be mutated through Vue.

`triggerRef(ref)` manually triggers updates on a `shallowRef` after an in-place mutation, when needed.

## When to reach for `reactive`

Reach for `reactive` when the state is a single object that lives for the lifetime of the component, has many properties, and is mutated in place rather than replaced. Even then, prefer `ref` if any consumer needs to destructure or if any property is a primitive at the top level. The Vue docs treat `reactive` as the secondary tool, not the default.

## References

- Reactivity Fundamentals: https://vuejs.org/guide/essentials/reactivity-fundamentals.html
- Reactivity API: https://vuejs.org/api/reactivity-core.html
- `toRefs`: https://vuejs.org/api/reactivity-utilities.html#torefs
- `shallowRef`: https://vuejs.org/api/reactivity-advanced.html#shallowref
