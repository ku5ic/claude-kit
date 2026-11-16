# Splitting work

- Cut vertically. A ticket for "the UI" and one for "the database" may be small, but they fail at independent and valuable.
- A slice changes what the system does and crosses layers as it needs to.
- Ways to split are workflow steps, operations (create, read, update, delete), business rule variations, data variations, data entry methods, the simple case first, deferring performance, or a spike.
- A spike's outcome is an answer. Use one when an unknown blocks the cut.
- One bug per ticket. A report naming several issues becomes several tickets.
- Many tickets from one source go under a parent, posted first.
