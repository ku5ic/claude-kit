---
name: graphql-patterns
description: GraphQL client patterns - operation and fragment authoring, typed documents with graphql-codegen's client preset, Apollo Client 4 caching and error handling, and review-time anti-patterns. Use whenever the project contains `.graphql`/`.gql` files, a `codegen.ts`, or `graphql`, `@apollo/client`, `urql`, or `graphql-request` in `package.json`, OR the user asks about GraphQL queries, mutations, fragments, the Apollo cache, or generated GraphQL types, even if GraphQL is not mentioned by name.
---

# GraphQL patterns

Default assumption: a TypeScript client on Apollo Client 4.3 with documents typed by `@graphql-codegen/client-preset` 6.x.

- Document rules (`reference/documents.md`) hold for every client: Apollo, urql, graphql-request.
- Apollo Client 3 projects: the v4 deltas (entry points, the unified `error`, `LocalState`) are called out in `reference/apollo-client.md`.
- Adapt advice to the version in the project's `package.json` or lockfile, and the `versions` line in `<repo-context>` when present.

## Reference files

| File                                                     | Covers                                                                                   |
| -------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| [reference/documents.md](reference/documents.md)         | Operation names, variables, fragments and colocation                                     |
| [reference/codegen.md](reference/codegen.md)             | Client preset config, the `graphql()` function, fragment masking                         |
| [reference/apollo-client.md](reference/apollo-client.md) | v4 entry points, error handling, `errorPolicy`, fetch policies, cache normalization      |
| [reference/anti-patterns.md](reference/anti-patterns.md) | Six review-time anti-patterns with severity calls                                        |

## References

- GraphQL queries and variables: https://graphql.org/learn/queries/
- Apollo Client docs: https://www.apollographql.com/docs/react/
- Apollo Client 4 migration: https://github.com/apollographql/apollo-client/blob/main/docs/source/migrating/apollo-client-4-migration.mdx
- GraphQL Code Generator client preset: https://the-guild.dev/graphql/codegen/plugins/presets/preset-client

## Version notes

Checked: 2026-09-26 against npm registry, Context7 /apollographql/apollo-client and /dotansimha/graphql-code-generator

- @apollo/client 4.3.1 (2026-09-18), 4.0.0 (2025-08-21): v4 moved React hooks to `@apollo/client/react`, replaced `ApolloError` with `CombinedGraphQLErrors` and a single `error` property, and moved local resolvers to `LocalState`.
- @graphql-codegen/client-preset 6.2.0 (2026-09-13), @graphql-codegen/cli 7.4.3 (2026-09-24): no guidance change.
- graphql (graphql-js) 17.0.2; 17.0.0 (2026-06-15) followed 16.14. Server-side changes are out of this pack's scope.
