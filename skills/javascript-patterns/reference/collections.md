# Collections: Map, Set, Weak variants

## `Map` over plain objects when keys are dynamic

Plain object keys are coerced to strings (or symbols), inherit from `Object.prototype`, and have no built-in iteration order guarantee for non-integer-like keys. `Map`:

- Accepts any value as a key (object identity preserved).
- Preserves insertion order for iteration.
- Has a `.size` accessor (`Object.keys(obj).length` is O(n)).
- Does not collide with prototype keys (`__proto__`, `constructor`).

Plain objects are still right when the keys are a known fixed set ("config-shaped"). Reach for `Map` when keys are user input, derived from data, or non-string.

## `Set` for unique collections

```js
const seen = new Set();
for (const item of items) {
  if (seen.has(item.id)) continue;
  seen.add(item.id);
  // ...
}
```

Cleaner than tracking presence in an object or array. `Set` membership check is O(1); array `.includes` is O(n).

## `WeakMap` and `WeakSet`

Keys must be objects and are held weakly: when the key has no other references, the entry is eligible for garbage collection. Use them to attach metadata to objects you do not own (DOM nodes, third-party class instances) without preventing them from being collected.

WeakMap and WeakSet are not iterable; this is by design (iteration order would expose GC timing).

## References

- MDN Map: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Map
- MDN Set: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Set
- MDN WeakMap: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/WeakMap
