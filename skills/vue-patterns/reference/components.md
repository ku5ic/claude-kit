# Components and `<script setup>`

- Prop defaults on 3.5+: Reactive Props Destructure, `const { msg = "hello", labels = ["one", "two"] } = defineProps<Props>()`. The bindings stay reactive, and mutable defaults need no factory.
- Prop defaults on 3.4 and earlier: `withDefaults(defineProps<Props>(), { msg: "hello", labels: () => ["one", "two"] })`, with a factory for each mutable default. `withDefaults` still works on 3.5; check the project's Vue version before recommending either form.
- A component `v-model` on 3.4+: `const model = defineModel<string>()`, read and written through `model.value`, in place of a `modelValue` prop plus an `update:modelValue` emit. A named `v-model:foo`: `defineModel<T>("foo")`.
- Typing emits on 3.3+: the tuple form, `defineEmits<{ change: [id: string]; update: [value: number, source: "input" | "blur"] }>()`. The older call-signature form, `(e: 'change', id: string): void`, still works.
- Typing slots on 3.3+: `defineSlots<{ default(props: { msg: string }): unknown; header?(props: { count: number }): unknown }>()`.
- A new SFC: `<script setup>`, not `defineComponent({ setup })`. Its top-level imports and bindings reach the template without a `return`, and the block runs once per component instance.
- Props in a TypeScript project: the type-only form, `defineProps<{ id: string; count?: number }>()`. The runtime form, `defineProps({ id: { type: String, required: true } })`, when you need prop validators.
