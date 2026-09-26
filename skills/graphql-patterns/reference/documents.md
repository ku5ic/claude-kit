# Documents: operations, variables, fragments

Rules for the GraphQL documents themselves, whatever client sends them.

## Name every operation

Write `query UserProfile($id: ID!) { ... }`, not an anonymous `{ ... }`. A name is required when a document holds several operations, and encouraged for a single one because it shows up in debugging and server-side logs. Source: https://graphql.org/learn/queries/

## Dynamic values go in variables

Pass runtime values as `$variables` declared on the operation and supplied in the client's `variables` object. Building the query string at runtime means the client has to manipulate and serialize GraphQL text itself, and a value spliced into the document becomes part of the document. Source: https://graphql.org/learn/queries/

Review check: a template literal holding a GraphQL document with `${...}` inside it is a finding, unless the interpolation is a fragment document (the `gql` tag's fragment composition), not a value.

## Fragments are reusable field sets

A fragment names a set of fields on a type and is spread wherever those fields are needed. Source: https://graphql.org/learn/queries/

- Colocate a component's fragment with the component that renders those fields, and compose the page query from the fragments. With codegen's client preset, fragment masking enforces this (`reference/codegen.md`).
- Select only the fields the component renders. A field added "in case" is a field every consumer of the fragment now fetches.

## Selections that feed a normalized cache

Select the type's identifier field (by default `id`, or whatever `keyFields` names) on every object you'll want updated in place. Apollo builds a cache ID from `__typename` plus the identifier fields. An object without its identifier can't be normalized and is embedded in its parent instead. Source: https://github.com/apollographql/apollo-client/blob/main/docs/source/caching/cache-configuration.mdx
