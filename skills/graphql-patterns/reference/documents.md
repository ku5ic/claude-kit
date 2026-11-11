# Documents: operations, variables, fragments

These hold whatever client sends the document.

- An operation: name it, as in `query UserProfile($id: ID!) { ... }`, even when it's the only one in the document. Never an anonymous `{ ... }`.
- A runtime value: declare it as a `$variable` on the operation and pass it in the client's `variables` object. Never build the document text at runtime.
- Review check: a template literal holding a GraphQL document with `${...}` inside it is a finding, unless the interpolation is a fragment document (the `gql` tag's fragment composition).
- A set of fields several places need: a fragment on the type, spread wherever those fields are needed.
- A component's fields: colocate its fragment with the component and compose the page query from the fragments. With codegen's client preset, fragment masking enforces this (`reference/codegen.md`).
- A fragment's selection: only the fields the component renders, none "in case".
- An object you'll want updated in place in a normalized cache: select its identifier field, `id` by default or whatever `keyFields` names.
