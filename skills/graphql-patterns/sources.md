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

### Left to linters

Checked: 2026-11-11 with ESLint 10.14.0 and `js.configs.recommended` from `@eslint/js` 10.0.1, and TypeScript 7.0.2 (`tsc --strict --noEmit`, no extra flags), with @apollo/client 4.3.8 and graphql 16.15.0, on a probe directory. No rule was dropped, because ESLint's defaults reported none of them on the JavaScript probe.

Not caught by defaults, so kept: a runtime value interpolated into a `gql` template, an anonymous operation, `getContext().getCacheKey` in a link.

Caught by tsc in TypeScript but kept for JavaScript clients: a hook imported from the `@apollo/client` root (`TS2305`), an `ApolloError` import (`TS2305`), `result.errors` (`TS2551`), `error.graphQLErrors` (`TS2339`), `resolvers` passed to `new ApolloClient` (`TS2353`), `fetchPolicy: "cache-and-network"` on `client.query` (`TS2769`).

## reference/anti-patterns.md

### Claims

- v3's `errors` and `ApolloError` on Apollo Client 4: https://github.com/apollographql/apollo-client/blob/main/docs/source/migrating/apollo-client-4-migration.mdx
- A runtime value interpolated into a document: https://graphql.org/learn/queries/
- An object selected without its identifier field: https://github.com/apollographql/apollo-client/blob/main/docs/source/caching/cache-configuration.mdx
- `errorPolicy: "all"` handled as all-or-nothing: https://github.com/apollographql/apollo-client/blob/main/docs/agent-skills/apollo-client/references/mutations.md
- `cache-and-network` passed to `client.query`: https://github.com/apollographql/apollo-client/blob/main/src/core/ApolloClient.ts
- Hand edits in the codegen output directory: https://github.com/dotansimha/graphql-code-generator/blob/master/website/src/pages/plugins/presets/preset-client.mdx

## reference/apollo-client.md

### Claims

- Apollo Client 4: the v4 migration guide, https://github.com/apollographql/apollo-client/blob/main/docs/source/migrating/apollo-client-4-migration.mdx, and the docs under https://github.com/apollographql/apollo-client/tree/main/docs/source (Context7 /apollographql/apollo-client)
- Hook entry point, the unified `error` and removed `ApolloError`, `useLazyQuery` rejection, `notifyOnNetworkStatusChange` default, `getCacheKey` to `cache.identify`: the migration guide above
- `CombinedGraphQLErrors.is`, `ErrorLink` placement: https://github.com/apollographql/apollo-client/blob/main/docs/source/data/error-handling.mdx
- `errorPolicy: "all"` partial data: https://github.com/apollographql/apollo-client/blob/main/docs/agent-skills/apollo-client/references/mutations.md
- `cache-and-network` invariant in `client.query`: https://github.com/apollographql/apollo-client/blob/main/src/core/ApolloClient.ts
- `keyFields`, `keyFields: false`: https://github.com/apollographql/apollo-client/blob/main/docs/source/caching/cache-configuration.mdx
- `cache.identify`: https://github.com/apollographql/apollo-client/blob/main/docs/source/caching/cache-interaction.mdx
- `merge` and `mergeObjects`: https://github.com/apollographql/apollo-client/blob/main/docs/source/caching/cache-field-behavior.mdx
- `LocalState`: https://github.com/apollographql/apollo-client/blob/main/docs/source/local-state/local-resolvers.mdx

## reference/codegen.md

### Claims

- graphql-codegen: the client preset: https://github.com/dotansimha/graphql-code-generator/blob/master/website/src/pages/plugins/presets/preset-client.mdx (Context7 /dotansimha/graphql-code-generator)

## reference/documents.md

### Claims

- Named operations: https://graphql.org/learn/queries/
- Runtime values in variables: https://graphql.org/learn/queries/
- Fragments as reusable field sets: https://graphql.org/learn/queries/
- Identifier fields for a normalized cache: https://github.com/apollographql/apollo-client/blob/main/docs/source/caching/cache-configuration.mdx
