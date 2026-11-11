---
name: graphql-patterns
description: GraphQL client patterns for operations, fragments, the codegen client preset, and Apollo Client 4. Use for .graphql or .gql files, codegen.ts, Apollo, urql, or GraphQL questions.
---

# GraphQL patterns

Default assumption: a TypeScript client on Apollo Client 4.3 with documents typed by `@graphql-codegen/client-preset` 7.x.

- An Apollo Client 3 project: the v4 deltas (entry points, the unified `error`, `LocalState`) are the lines marked v4 in `reference/apollo-client.md`.
- Document rules (`reference/documents.md`) hold for every client: Apollo, urql, graphql-request.
- Adapt advice to the version in the project's `package.json` or lockfile.

## Reference files

| File                                                     | Covers                                                                               |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| [reference/documents.md](reference/documents.md)         | Operation names, variables, fragments and colocation, identifier fields              |
| [reference/codegen.md](reference/codegen.md)             | Client preset config, the `graphql()` function, fragment masking                     |
| [reference/apollo-client.md](reference/apollo-client.md) | v4 entry points, errors, and `LocalState`; `errorPolicy`, fetch policies, cache keys |
| [reference/anti-patterns.md](reference/anti-patterns.md) | Six review-time anti-patterns with severity calls                                    |
