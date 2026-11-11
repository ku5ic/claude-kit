# Collections: Map, Set, Weak variants

- Keys from user input or data, or non-string keys: `Map`. For keys from a known fixed set, use a plain object.
- `Map` keys any value by identity, iterates in insertion order, has `.size`, and never collides with `__proto__` or `constructor`.
- Unique values or a seen-check: `Set`, with O(1) `has`, not an array's O(n) `includes`.
- Metadata on objects you don't own (DOM nodes, third-party instances): `WeakMap` or `WeakSet`. Keys must be objects, an entry doesn't keep its key from garbage collection, and neither is iterable.
