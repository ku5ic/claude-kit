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

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have.

| Before                                                                                | Read                                                     |
| ------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| Writing or changing an operation, a variable, or a fragment                           | [reference/documents.md](reference/documents.md)         |
| Configuring codegen, or using the `graphql()` function or fragment masking            | [reference/codegen.md](reference/codegen.md)             |
| Writing or changing Apollo Client code: imports, errors, fetch policies, or the cache | [reference/apollo-client.md](reference/apollo-client.md) |
| Reviewing GraphQL client code                                                         | [reference/anti-patterns.md](reference/anti-patterns.md) |
