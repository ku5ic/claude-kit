# graphql-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- GraphQL queries and variables: https://graphql.org/learn/queries/
- Apollo Client docs: https://www.apollographql.com/docs/react/
- Apollo Client 4 migration: https://github.com/apollographql/apollo-client/blob/main/docs/source/migrating/apollo-client-4-migration.mdx
- GraphQL Code Generator client preset: https://the-guild.dev/graphql/codegen/plugins/presets/preset-client

### Version notes

Checked: 2026-11-10 against npm registry, Context7 /apollographql/apollo-client, and https://github.com/dotansimha/graphql-code-generator/blob/master/packages/presets/client/CHANGELOG.md

- @graphql-codegen/client-preset 7.0.0 and @graphql-codegen/cli 8.0.0 (2026-11-07): drop GraphQL 14 and 15 (peer `^16 || ^17`), need Node 22.18+, remove `emitLegacyCommonJSImports` (use `importExtension`). @apollo/client 4.3.8 (2026-11-09), graphql 17.1.0 (2026-11-04): no guidance change.
- @apollo/client 4.0.0 (2025-08-21): v4 moved React hooks to `@apollo/client/react`, replaced `ApolloError` with `CombinedGraphQLErrors` and a single `error` property, and moved local resolvers to `LocalState`.
- @graphql-codegen/client-preset 6.2.0 (2026-09-13), @graphql-codegen/cli 7.4.3 (2026-09-24): no guidance change.
- graphql (graphql-js) 17.0.2; 17.0.0 (2026-06-15) followed 16.14. Server-side changes are out of this pack's scope.

## reference/anti-patterns.md

### Claims

- A runtime value interpolated into a document, `failure`. `` gql`query { user(id: "${id}") { name } }` `...: https://graphql.org/learn/queries/
- An object selected without its identifier field, `warning`. Without `id` (or the type's `keyFields`), Apol...: https://github.com/apollographql/apollo-client/blob/main/docs/source/caching/cache-configuration.mdx
- Reading v3's `errors` or `ApolloError` on Apollo Client 4, `failure`. v4 removed both, so `error.graphQLErrors` chec...: https://github.com/apollographql/apollo-client/blob/main/docs/source/migrating/apollo-client-4-migration.mdx
- `errorPolicy: "all"` handled as all-or-nothing, `warning`. With `"all"`, data and errors come back togeth...: https://github.com/apollographql/apollo-client/blob/main/docs/agent-skills/apollo-client/references/mutations.md
- `cache-and-network` passed to `client.query`, `failure`. `client.query` returns one result; the policy...: https://github.com/apollographql/apollo-client/blob/main/src/core/ApolloClient.ts
- Hand edits in the codegen output directory, `failure`. The client preset regenerates it from `codegen...: https://github.com/dotansimha/graphql-code-generator/blob/master/website/src/pages/plugins/presets/preset-client.mdx

## reference/apollo-client.md

### Claims

- Apollo Client 4: the v4 migration guide, https://github.com/apollographql/apollo-client/blob/main/docs/source/migrating/apollo-client-4-migration.mdx, and the docs under https://github.com/apollographql/apollo-client/tree/main/docs/source (Context7 /apollographql/apollo-client)

## reference/codegen.md

### Claims

- graphql-codegen: the client preset: https://github.com/dotansimha/graphql-code-generator/blob/master/website/src/pages/plugins/presets/preset-client.mdx (Context7 /dotansimha/graphql-code-generator)

## reference/documents.md

### Claims

- Name every operation, Write `query UserProfile($id: ID!) { ... }`, not an anony...: https://graphql.org/learn/queries/
- Dynamic values go in variables, Pass runtime values as `$variables` declared on the opera...: https://graphql.org/learn/queries/
- Fragments are reusable field sets, A fragment names a set of fields on a type and is spread...: https://graphql.org/learn/queries/
- Selections that feed a normalized cache, Select the type's identifier field (by default `id`, or w...: https://github.com/apollographql/apollo-client/blob/main/docs/source/caching/cache-configuration.mdx
