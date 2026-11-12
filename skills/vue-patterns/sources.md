# vue-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- Vue docs: https://vuejs.org/guide/introduction.html
- Vue API reference: https://vuejs.org/api/
- Vue 3 changelog: https://github.com/vuejs/core/blob/main/CHANGELOG.md
- Pinia: https://pinia.vuejs.org/
- Vue blog (releases): https://blog.vuejs.org/
- Anthony Fu (core team, Vite/Nuxt contributor): https://antfu.me/
- Eduardo San Martin Morote (Vue Router, Pinia maintainer): https://esm.dev/

### Version notes

Checked: 2026-11-10 against https://github.com/vuejs/core/releases/latest (3.5.45, 2026-11-05; 3.6 still in release candidates), https://vuejs.org/guide/essentials/watchers.html and https://vuejs.org/api/sfc-script-setup.html

- 3.5 (2024-09-01): Reactive Props Destructure is the recommended default-value form over `withDefaults` (which still works); `onWatcherCleanup`.
- 3.4: `defineModel`; watcher `once: true`.
- 3.3: `defineSlots`; tuple-style `defineEmits` types.

### Left to linters

Checked: 2026-11-11 with TypeScript 7.0.2 (`tsc --strict --noEmit`, no extra flags) and ESLint 10.14.0 with `@eslint/js` 10.0.1 `js.configs.recommended`, on a probe project with Vue 3.5.45. No rule was dropped, because no default check reports the form a rule targets.

Not caught by defaults, so kept: destructuring a `reactive()` object, swapping a `reactive()` object, in-place prop mutation, an async watcher without cleanup, `deep: true`, `v-if` with `v-for`, `v-for` without a `key`, `v-html`, a provided unwrapped value.

Caught by a default but kept: `reactive("hello")` (tsc `TS2345`) and reassigning a prop (tsc `TS2540`) fire only in a `.ts` file. Plain tsc rejects `.vue` files with `TS6054`, and default ESLint ignores them ("no matching configuration").

## reference/anti-patterns.md

### References

- `v-for` with `v-if`: https://vuejs.org/style-guide/rules-essential.html#avoid-v-if-with-v-for
- `v-for` keys: https://vuejs.org/api/built-in-directives.html#v-for
- One-way data flow: https://vuejs.org/guide/components/props.html#one-way-data-flow
- Reactivity destructuring caveats: https://vuejs.org/guide/essentials/reactivity-fundamentals.html#limitations-of-reactive
- `v-html` security: https://vuejs.org/api/built-in-directives.html#v-html

## reference/components.md

### References

- `<script setup>`: https://vuejs.org/api/sfc-script-setup.html
- `defineProps` / `defineEmits` / `defineSlots`: https://vuejs.org/api/sfc-script-setup.html#defineprops-defineemits
- `defineModel`: https://vuejs.org/api/sfc-script-setup.html#definemodel
- Reactive Props Destructure (3.5+): https://vuejs.org/guide/components/props.html#reactive-props-destructure
- Vue 3 changelog: https://github.com/vuejs/core/blob/main/CHANGELOG.md

## reference/reactivity.md

### References

- Reactivity Fundamentals: https://vuejs.org/guide/essentials/reactivity-fundamentals.html
- Reactivity API: https://vuejs.org/api/reactivity-core.html
- `toRefs`: https://vuejs.org/api/reactivity-utilities.html#torefs
- `shallowRef`: https://vuejs.org/api/reactivity-advanced.html#shallowref

## reference/state.md

### References

- Provide / Inject: https://vuejs.org/guide/components/provide-inject.html
- Pinia: https://pinia.vuejs.org/
- Composables: https://vuejs.org/guide/reusability/composables.html

## reference/watchers.md

### References

- Watchers guide: https://vuejs.org/guide/essentials/watchers.html
- `watch`: https://vuejs.org/api/reactivity-core.html#watch
- `watchEffect`: https://vuejs.org/api/reactivity-core.html#watcheffect
