# State management

## `useState` for SSR-safe shared state

`useState(key, init)` creates a reactive ref that is per-request on the server and shared across consumers within the same request. The value is serialized into the page payload and rehydrated on the client; subsequent component instances within the same render reuse the same ref.

```ts
const counter = useState("counter", () => 0);
counter.value++;
```

The key is required (or auto-generated from the call site, but explicit is safer). The init function runs once per request on the server, then the value comes from the payload on the client.

## Module-level `ref` / `reactive` is server-side cross-request leakage

`failure`. A ref declared at the module top level lives in the JS module instance, which is shared across every request the server handles in the same process:

```ts
// composables/useUser.ts
import { ref } from "vue";
const currentUser = ref<User | null>(null); // shared across ALL server requests

export function useUser() {
  return currentUser;
}
```

The first user to log in sets `currentUser`; the second user's request sees the first user's data. The fix is `useState`:

```ts
// composables/useUser.ts
export function useUser() {
  return useState<User | null>("currentUser", () => null);
}
```

The same module-level pattern is fine in client-side SPAs (one JS context per browser tab), but Nuxt SSR shares the process; treat module-level reactive state as a defect.

## Pinia for application-wide state

Pinia is the recommended state library for Nuxt apps. Install `@pinia/nuxt` and Pinia stores work across server and client; serialization to the payload is handled automatically. Use Pinia when:

- The same state is read from multiple components across the route tree.
- The state needs derived getters and shared mutations (Pinia actions).
- DevTools introspection matters.

For state that lives inside one composable used by a few components, plain `useState` plus a composable wrapper is lighter than reaching for Pinia.

## Composables are the unit of reuse

Composables (functions returning reactive state) are the right boundary for shared logic. In Nuxt, composables under `composables/` are auto-imported in components.

```ts
// composables/useCart.ts
export function useCart() {
  const items = useState<CartItem[]>("cart-items", () => []);
  const total = computed(() =>
    items.value.reduce((sum, i) => sum + i.price, 0),
  );
  function add(item: CartItem) {
    items.value = [...items.value, item];
  }
  return { items, total, add };
}
```

The internal `useState` makes this SSR-safe. Without it, this composable would have the cross-request-leakage problem above.

## References

- `useState`: https://nuxt.com/docs/api/composables/use-state
- State management guide: https://nuxt.com/docs/getting-started/state-management
- Pinia for Nuxt: https://pinia.vuejs.org/ssr/nuxt.html
- Composables auto-import: https://nuxt.com/docs/guide/directory-structure/composables
