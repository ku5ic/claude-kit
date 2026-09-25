# Anti-patterns

Severity rubric:

- `failure`: a concrete defect or violation that should not ship.
- `warning`: a smell or pattern that compounds with other findings.
- `info`: a hardening opportunity or note, not a defect.

## `v-if` and `v-for` on the same element

`failure`. Vue documents this combination as a precedence trap: the two directives' relative priority is non-obvious and the resulting evaluation order is rarely what the author intended. Move `v-if` to a wrapper element (often `<template>`), or filter the array with a computed before the `v-for`.

```vue
<!-- failure -->
<li v-for="item in items" v-if="item.visible">...</li>

<!-- correct: filter first -->
<template v-for="item in visibleItems" :key="item.id">
  <li>...</li>
</template>
```

## Missing `key` on `v-for`

`warning`. Without a stable key, Vue falls back to in-place patching by index, which produces input-state mixups, focus jumps, and bleeding controlled-input values when the list reorders. Always set `:key="item.id"` to a stable id from the data.

## Destructuring `reactive` at the top of `<script setup>`

`failure`. The destructured bindings lose their connection to the proxy and stop updating. Use `toRefs(state)` to preserve reactivity, or reach for `ref()` in the first place. Reactive props destructure (Vue 3.5+) is a special case: that destructure pattern IS reactive because the compiler rewrites it. Plain destructure of an arbitrary `reactive(...)` object is not.

## `v-html` with user input

`failure`. Renders the string as raw HTML, bypassing Vue's escaping. Direct XSS vector. Either render as text (`{{ value }}`), restrict to trusted content, or sanitize with DOMPurify before binding.

## Mutating props in a child component

`failure`. Vue's data flow is one-way: parent owns the prop, child receives a snapshot. A child that mutates `props.user.role` produces inconsistent state across components and triggers a runtime warning. Emit an event for the parent to apply (`emit('update:role', newRole)`), or use `defineModel` for the explicit two-way contract.

## Deep watchers as the default

`warning`. `{ deep: true }` traverses the entire reactive tree to set up tracking and re-traverses on each mutation. On large objects this becomes the dominant cost. Watch a specific computed or a path expression instead, or use `watchEffect` and let auto-tracking pick only the touched fields.

## Using `reactive(primitive)`

`warning`. `reactive("hello")` returns the string unchanged; the value is not made reactive. For primitives, use `ref()`. Some linters catch this; if not, code-review for it explicitly.

## `watch` without cleanup for async work

`warning`. An async watcher that does not cancel its in-flight request when the dep changes will race: the second response can arrive after the third, and the wrong value lands in state. Use `onWatcherCleanup()` (Vue 3.5+) or the `onCleanup` callback argument to abort superseded requests with `AbortController`.

## Mutating `props` array / object via method calls

`failure`. `props.items.push(...)` and `props.config.x = ...` are subtler forms of prop mutation. Vue's runtime warning catches reassignment but not in-place mutation. Treat all props as immutable; if the parent must apply a change, emit it.

## References

- `v-for` with `v-if`: https://vuejs.org/style-guide/rules-essential.html#avoid-v-if-with-v-for
- `v-for` keys: https://vuejs.org/api/built-in-directives.html#v-for
- One-way data flow: https://vuejs.org/guide/components/props.html#one-way-data-flow
- Reactivity destructuring caveats: https://vuejs.org/guide/essentials/reactivity-fundamentals.html#limitations-of-reactive
- `v-html` security: https://vuejs.org/api/built-in-directives.html#v-html
