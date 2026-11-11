---
name: vue-patterns
description: Vue 3 patterns for the Composition API, reactivity, and single-file components. Use for .vue files, vue in package.json, a Vite config with the Vue plugin, or Vue questions.
---

# Vue patterns

Default assumption: Vue 3 with the Composition API and `<script setup>`.

- Current stable is Vue 3.5. Rules for 3.5+ (Reactive Props Destructure, `onWatcherCleanup`) and 3.4+ (`defineModel`, `once`) carry their version.
- Match advice to the Vue version in the project's `package.json` or lockfile.
- Options API code: acceptable as legacy; flag it as legacy in review.

## Reference files

| File                                                     | Covers                                                                                    |
| -------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| [reference/reactivity.md](reference/reactivity.md)       | `ref` vs `reactive`, `toRefs`, `shallowRef`, destructuring traps                          |
| [reference/components.md](reference/components.md)       | Defaults (3.5+ vs 3.4), `defineModel` (3.4+), `defineEmits`/`Slots` (3.3+), `defineProps` |
| [reference/watchers.md](reference/watchers.md)           | `onWatcherCleanup` (3.5+), `once` (3.4+), `watch` vs `watchEffect`, deep watchers         |
| [reference/state.md](reference/state.md)                 | `provide`/`inject`, Pinia, composables                                                    |
| [reference/anti-patterns.md](reference/anti-patterns.md) | Nine review-time anti-patterns with severity calls                                        |

## Out of scope

- A Vue 2 project: flag the project's Vue major and stop. Its reactivity model differs and these patterns mislead.
- A React project, despite the surface likeness (component tree, hooks-like primitives). The reactivity models differ.
