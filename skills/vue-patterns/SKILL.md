---
name: vue-patterns
description: Vue 3 patterns for the Composition API, reactivity, and single-file components. Use for .vue files, vue in package.json, a Vite config with the Vue plugin, or Vue questions.
---

# Vue patterns

Default assumption: Vue 3 with the Composition API and `<script setup>` syntax.

- Current stable is Vue 3.5; deltas in 3.4+ (`defineModel`) and 3.5+ (Reactive Props Destructure, `onWatcherCleanup`) are called out where they matter.
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
