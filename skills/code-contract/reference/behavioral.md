# Breaks no type diff sees

- With enough users, every observable behavior is depended on by somebody, so "outside the contract" isn't safe. Raise the class when the surface has many consumers.
- A new required field in a request breaks existing clients.
- A changed default is a break.
- A changed field type is a break, and a rename is a removal plus an addition.
- A change to visible behavior that breaks reasonable caller code is a break.
- A new value in a response enum may go unhandled by callers. Class it minor and flag it.
- In Rust, adding a trait item without a default, or changing a function's arguments, is major. Adding a defaulted item is minor.
- In Go, a new struct field breaks unkeyed struct literals. The Go 1 promise exempts this, and bug and security fixes, so class them minor with a note.
