# State management

- Cross-cutting state such as theme, locale, or current user goes through `provide` in an ancestor and `inject` in any descendant. Data local to a component tree stays in props.
- A provided value that must stay reactive: provide the ref or computed, not its unwrapped value. To stop descendants from mutating it, wrap it in `readonly()`.
- Injection keys in TypeScript: a Symbol typed `InjectionKey<T>`, as in `export const ThemeKey: InjectionKey<Ref<"light" | "dark">> = Symbol("theme")`. `inject(ThemeKey)` is then typed, and string keys can't collide.
- App-wide state that several distant consumers read or mutate: a Pinia store, not Vuex, which is in maintenance. Define it with `defineStore("user", () => { ...; return { user, signIn } })` and consume it through the returned `useUserStore()`.
- A `ref` declared at module scope is one instance shared by every consumer in the same JS context. That's fine in a client-side SPA, but on a server it leaks across requests (the Nuxt skill covers SSR).
- Logic shared across components: a composable, a `useXxx` function that uses `ref`, `computed`, `watch`, or lifecycle hooks and returns reactive state. Reach for one before a Pinia store; many shared-logic problems need no cross-component state.
- A composable that adds an event listener in `onMounted`: remove it in `onUnmounted`.
