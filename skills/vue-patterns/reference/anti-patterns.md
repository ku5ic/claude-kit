# Anti-patterns

Severity levels: the report-format skill.

- An async watcher that doesn't cancel its in-flight request when the dependency changes: `warning`. Abort the superseded request with an `AbortController`, from `onWatcherCleanup()` on 3.5+ or the `onCleanup` callback argument.
- `v-if` and `v-for` on the same element: `failure`. Move `v-if` to a wrapper element, often `<template>`, or filter the array with a computed and `v-for` over that.
- `v-for` without a `key`: `warning`. Set `:key` to a stable id from the data, as in `:key="item.id"`.
- Destructuring a `reactive()` object at the top of `<script setup>`: `failure`. Use `toRefs(state)`, or `ref()` from the start. Reactive Props Destructure of `defineProps()` (3.5+) is the exception; the compiler keeps it reactive.
- `v-html` with user input: `failure`, a direct XSS vector. Render it as text with `{{ value }}`, restrict it to trusted content, or sanitize it with DOMPurify first. The rest of the XSS surface: security-patterns' `reference/frontend.md`.
- A child component mutating a prop, as in `props.user.role = ...`: `failure`. Emit an event for the parent to apply, `emit('update:role', newRole)`, or use `defineModel` for an explicit two-way contract.
- `{ deep: true }` as the default watcher option: `warning`. Watch a specific computed or a path getter, or let `watchEffect` auto-track only the touched fields.
- `reactive(primitive)`: `warning`. `reactive("hello")` returns the string unchanged; use `ref()`.
- Mutating a prop array or object in place, as in `props.items.push(...)` or `props.config.x = ...`: `failure`. Vue's runtime warning catches reassignment, not in-place mutation. Treat every prop as immutable and emit the change for the parent.
