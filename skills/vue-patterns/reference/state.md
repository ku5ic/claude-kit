# State management

## `provide` / `inject` for cross-cutting state

`provide` and `inject` solve prop drilling: a parent provides a value (any value, including reactive refs and computeds) and any descendant injects it without intermediate components needing to know.

```ts
// parent
import { provide, ref } from "vue";
const theme = ref<"light" | "dark">("light");
provide("theme", theme);

// any descendant
import { inject } from "vue";
const theme = inject<Ref<"light" | "dark">>("theme");
```

For reactivity to survive the boundary, provide a ref or computed (not the unwrapped value). To prevent descendants from mutating, wrap in `readonly()`.

For typed injection in TypeScript projects (and to avoid string-key collisions), use a Symbol with an `InjectionKey<T>` type:

```ts
import { provide, inject, type InjectionKey, type Ref } from "vue";
export const ThemeKey: InjectionKey<Ref<"light" | "dark">> = Symbol("theme");

provide(ThemeKey, theme);
const theme = inject(ThemeKey); // typed automatically
```

Reach for `provide` / `inject` when the value is genuinely cross-cutting (theme, locale, current user). For component-tree-local data, props remain cleaner; the explicit data flow is part of why props are easy to debug.

## Pinia for app-wide state

Pinia is the official, current state-management library for Vue. It replaces Vuex in modern Vue projects (Vuex is in maintenance). A store is defined with `defineStore(id, setupFn)` and consumed via the returned composable:

```ts
import { defineStore } from "pinia";
import { ref, computed } from "vue";

export const useUserStore = defineStore("user", () => {
  const user = ref<User | null>(null);
  const isAuthenticated = computed(() => user.value !== null);

  async function signIn(credentials: Credentials) {
    user.value = await api.signIn(credentials);
  }

  return { user, isAuthenticated, signIn };
});

// in any component:
const userStore = useUserStore();
userStore.signIn(creds);
```

Pinia stores are typed end-to-end, integrate with Vue DevTools, and survive HMR. Reach for Pinia when the state genuinely belongs to the application (not a single component subtree) and when multiple distant consumers need to read or mutate it.

For module-level singleton state inside an SFC import (a `ref` declared at module scope), be aware that the same module instance is shared across every consumer in the same JS context. That is fine in client-side SPAs but causes cross-request leakage on a server (see Nuxt patterns for the SSR case).

## Composables: reusable reactive logic

A composable is a function that uses Composition-API primitives (`ref`, `computed`, `watch`, lifecycle hooks) and returns reactive state. Naming convention: `useXxx`.

```ts
import { ref, onMounted, onUnmounted } from "vue";

export function useMouse() {
  const x = ref(0);
  const y = ref(0);

  function update(event: MouseEvent) {
    x.value = event.pageX;
    y.value = event.pageY;
  }

  onMounted(() => window.addEventListener("mousemove", update));
  onUnmounted(() => window.removeEventListener("mousemove", update));

  return { x, y };
}
```

Composables are the right unit for sharing logic across components. They are testable in isolation, declarative, and naturally compose. Reach for them before extracting state into a Pinia store; many "shared logic" problems do not need cross-component state.

## References

- Provide / Inject: https://vuejs.org/guide/components/provide-inject.html
- Pinia: https://pinia.vuejs.org/
- Composables: https://vuejs.org/guide/reusability/composables.html
