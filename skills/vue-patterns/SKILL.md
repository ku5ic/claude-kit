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

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have.

| Before                                                                                                    | Read                                                     |
| --------------------------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| Creating reactive state with `ref`, `reactive`, `toRefs`, or `shallowRef`, or destructuring reactive data | [reference/reactivity.md](reference/reactivity.md)       |
| Writing a component's props, emits, slots, defaults, or `defineModel`                                     | [reference/components.md](reference/components.md)       |
| Writing a `watch` or `watchEffect`, a deep watcher, or watcher cleanup                                    | [reference/watchers.md](reference/watchers.md)           |
| Sharing state with `provide`/`inject`, a Pinia store, or a composable                                     | [reference/state.md](reference/state.md)                 |
| Reviewing Vue code                                                                                        | [reference/anti-patterns.md](reference/anti-patterns.md) |

## Out of scope

- A Vue 2 project: flag the project's Vue major and stop. Its reactivity model differs and these patterns mislead.
- A React project, despite the surface likeness (component tree, hooks-like primitives). The reactivity models differ.
