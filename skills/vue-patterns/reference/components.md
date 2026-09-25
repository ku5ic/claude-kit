# Components and `<script setup>`

## `<script setup>` over `defineComponent({ setup })`

`<script setup>` is the recommended SFC syntax for Vue 3. The Vue docs cite four reasons over the explicit `defineComponent({ setup })` form:

- More succinct code; no manual `return { ... }` to expose bindings.
- Better TypeScript inference for props and emits via type-only declarations.
- Better runtime performance: the template compiles into a render function in the same scope, without an intermediate proxy.
- Better IDE / language-server performance.

Top-level imports and bindings inside `<script setup>` are automatically available in the template. The block runs once per component instance.

## `defineProps`

Compiler macro. Two forms; prefer the type-only form in TypeScript projects:

```ts
// Type-only (preferred in TS)
const props = defineProps<{
  id: string;
  count?: number;
}>();

// Runtime form (useful for prop validators)
const props = defineProps({
  id: { type: String, required: true },
  count: { type: Number, default: 0 },
});
```

## `defineEmits`

Compiler macro. The Vue 3.3+ tuple-style type form is the cleanest in TypeScript:

```ts
const emit = defineEmits<{
  change: [id: string];
  update: [value: number, source: "input" | "blur"];
}>();

emit("update", 42, "input");
```

The older call-signature form (`(e: 'change', id: string): void`) still works.

## `defineSlots` (Vue 3.3+)

Compiler macro for typing slot content. Available in 3.3 and later:

```ts
const slots = defineSlots<{
  default(props: { msg: string }): unknown;
  header?(props: { count: number }): unknown;
}>();
```

This narrows what consumers can pass and gives the template editor type-aware slot completion.

## Default values: Reactive Props Destructure (3.5+) over `withDefaults`

In Vue 3.5+, the recommended way to give props default values is to destructure `defineProps()` and assign defaults in the destructure:

```ts
// Vue 3.5+: Reactive Props Destructure
const { msg = "hello", labels = ["one", "two"] } = defineProps<Props>();
```

The destructured bindings remain reactive. Mutable defaults (arrays, objects) do not need to be wrapped in factory functions in this form.

In Vue 3.4 and earlier, use `withDefaults`:

```ts
const props = withDefaults(defineProps<Props>(), {
  msg: "hello",
  labels: () => ["one", "two"], // factory required for mutable defaults in this form
});
```

`withDefaults` still works in 3.5; the destructure form is the new preference. Verify the project's Vue version before recommending one over the other.

## `defineModel` (Vue 3.4+)

`v-model` on components used to require a `modelValue` prop and an `update:modelValue` emit per binding. `defineModel()` collapses both:

```ts
const model = defineModel<string>();
// reads via model.value, writes via model.value = ...
// parent uses <Child v-model="parentRef" />
```

Multiple `v-model:foo` bindings: `defineModel<T>("foo")`.

## References

- `<script setup>`: https://vuejs.org/api/sfc-script-setup.html
- `defineProps` / `defineEmits` / `defineSlots`: https://vuejs.org/api/sfc-script-setup.html#defineprops-defineemits
- `defineModel`: https://vuejs.org/api/sfc-script-setup.html#definemodel
- Reactive Props Destructure (3.5+): https://vuejs.org/guide/components/props.html#reactive-props-destructure
- Vue 3 changelog: https://github.com/vuejs/core/blob/main/CHANGELOG.md
