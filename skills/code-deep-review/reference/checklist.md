# What a reviewer checks beyond the stack and the tests

- Functionality: does it do what the author meant, for its users and for later developers? Edge cases, concurrency, and deadlocks or races in parallel code.
- Design: do the parts interact sensibly, does it belong in this codebase or a library, and does it fit the system?
- Documentation: a change to how the code is built, tested, used, or released updates the matching docs. Docs for deleted or deprecated code go.
- Comments: a stale TODO is updated or removed. A comment says why, not what.
- Every human-written line in the target is read. Generated code and large data files can be skimmed.
- A part you can't judge, such as security or concurrency, is named in the report as needing a qualified reviewer.
- A partial review says which parts it covered.
- A good practice or a clean fix in the target is named.
