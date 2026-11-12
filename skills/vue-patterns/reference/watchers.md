# Watchers

- Async work in a watcher on 3.5+: create an `AbortController`, register `onWatcherCleanup(() => controller.abort())`, and pass `controller.signal` to the request. Older versions: the third callback argument, `watch(source, async (val, _old, onCleanup) => { onCleanup(...) })`. Both forms still work.
- A callback that should run at most once, on 3.4+: `once: true`.
- One specific ref or computed as the dependency, a need for the previous value, or a heavy callback that should run only on real changes: `watch(source, (newValue, oldValue) => ...)`. It fires only when that source changes.
- Several dependencies that trigger one effect, a dependency set awkward to enumerate, or immediate execution: `watchEffect`. It tracks every reactive value read inside it, runs once immediately, then re-runs when any of them changes.
- A `watch` callback that should run on setup: `immediate: true`. `watchEffect` already does.
- A callback that should run after Vue updates the DOM: `flush: 'post'`. Synchronously, before the next microtask: `flush: 'sync'`. The default, `flush: 'pre'`, runs before the DOM update in the same tick.
- Use `deep: true`, or `watchEffect` over a deeply nested path, only as a last resort. Watch a computed that derives only the field you need, a path getter such as `watch(() => state.user.email, ...)`, or a field moved to the top level. Where it fits, let `watchEffect` auto-track only the touched fields.
