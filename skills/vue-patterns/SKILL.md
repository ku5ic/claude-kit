---
name: vue-patterns
description: Vue 3 patterns - the Composition API, reactivity, single-file components, anti-patterns, and review checklist. Use whenever the project contains `.vue` files, `vue` in `package.json`, or a `vite.config.*` with the Vue plugin, OR the user asks about Vue, its reactivity primitives, `script setup`, or Pinia, even if Vue is not mentioned by name.
---

# Vue patterns

Default assumption: Vue 3 with the Composition API and `<script setup>` syntax.

- Current stable is Vue 3.5; deltas in 3.5+ (Reactive Props Destructure, `onWatcherCleanup`, `defineModel`) are called out where they matter.
- Options API is acceptable for legacy code; flag as legacy when reviewed.
- Adapt advice to the version in the project's `package.json` or lockfile.

## Reference files

| File                                                     | Covers                                                                          |
| -------------------------------------------------------- | ------------------------------------------------------------------------------- |
| [reference/reactivity.md](reference/reactivity.md)       | `ref` vs `reactive`, `toRefs`, `shallowRef`, destructuring traps                |
| [reference/components.md](reference/components.md)       | `<script setup>`, `defineProps`/`Emits`/`Slots`/`Model`, defaults (3.5+ vs 3.4) |
| [reference/watchers.md](reference/watchers.md)           | `watch` vs `watchEffect`, options, deep watchers, async cleanup                 |
| [reference/state.md](reference/state.md)                 | `provide`/`inject`, Pinia, composables                                          |
| [reference/anti-patterns.md](reference/anti-patterns.md) | Nine review-time anti-patterns with severity calls                              |

## Out of scope

- Vue 2 projects. The Options API is the default and the reactivity model is fundamentally different. Flag the project's Vue major and stop; the patterns here will mislead.
- React projects. Despite surface similarities (component tree, hooks-like primitives), the reactivity models differ.

## References

- Vue docs: https://vuejs.org/guide/introduction.html
- Vue API reference: https://vuejs.org/api/
- Vue 3 changelog: https://github.com/vuejs/core/blob/main/CHANGELOG.md
- Pinia: https://pinia.vuejs.org/
- Vue blog (releases): https://blog.vuejs.org/
- Anthony Fu (core team, Vite/Nuxt contributor): https://antfu.me/
- Eduardo San Martin Morote (Vue Router, Pinia maintainer): https://esm.dev/

## Version notes

Checked: 2026-09-12 against Context7 /vuejs/vue (3.5.42) and https://vuejs.org/api/sfc-script-setup.html

- 3.5 (2024-09-01): Reactive Props Destructure is the recommended default-value form over `withDefaults` (which still works); `onWatcherCleanup`.
- 3.4: `defineModel`; watcher `once: true`.
- 3.3: `defineSlots`; tuple-style `defineEmits` types.
