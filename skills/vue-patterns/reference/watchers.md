# Watchers

## `watch` vs `watchEffect`

`watch(source, callback)` tracks a single explicit source. The callback fires only when that source changes and receives `(newValue, oldValue)`. Use it when:

- The dependency is one specific ref or computed.
- You need the previous value.
- The callback is heavy and you want it to run only on real changes.

`watchEffect(callback)` tracks every reactive value accessed inside the callback. It runs once immediately, then re-runs whenever any tracked dependency changes. Use it when:

- Multiple dependencies should all trigger the same effect.
- The dependency set is awkward to enumerate.
- You want immediate execution without `{ immediate: true }`.

```ts
watch(userId, async (id) => {
  user.value = await fetchUser(id);
});

watchEffect(async () => {
  user.value = await fetchUser(userId.value); // dep auto-tracked
});
```

## Options

- `immediate: true` (`watch` only): run the callback immediately on setup. `watchEffect` already does this.
- `deep: true`: recursively track nested properties of the source. Expensive (see below).
- `flush: 'post'`: defer the callback until after Vue updates the DOM.
- `flush: 'sync'`: run the callback synchronously, before the next microtask.
- `once: true` (Vue 3.4+): the callback runs at most once.

`flush: 'pre'` is the default; the callback runs before the DOM update for the same tick.

## Deep watchers are expensive

`{ deep: true }` (or `watchEffect` over a deeply nested path) recursively walks the entire reactive tree to set up tracking. Every nested property is tracked; every mutation re-traverses on the way out. On large objects this becomes the dominant cost in a render.

Cheaper alternatives:

- Watch a specific computed that derives only the field you care about.
- Watch the path explicitly: `watch(() => state.user.email, ...)`.
- Restructure the data so the watched field is at the top level.

Treat `{ deep: true }` as a last resort; reach for `watchEffect` and let auto-tracking pick only the touched fields when possible.

## Cleanup with `onWatcherCleanup`

For async work inside a watcher, register cleanup with `onWatcherCleanup` (Vue 3.5+) so a stale request can be cancelled when a new dep change supersedes it:

```ts
import { watch, onWatcherCleanup } from "vue";

watch(query, async (q) => {
  const controller = new AbortController();
  onWatcherCleanup(() => controller.abort());
  const res = await fetch(`/search?q=${q}`, { signal: controller.signal });
  results.value = await res.json();
});
```

Older Vue versions used the third callback argument: `watch(source, async (val, _old, onCleanup) => { onCleanup(...) })`. Both forms still work.

## References

- Watchers guide: https://vuejs.org/guide/essentials/watchers.html
- `watch`: https://vuejs.org/api/reactivity-core.html#watch
- `watchEffect`: https://vuejs.org/api/reactivity-core.html#watcheffect
