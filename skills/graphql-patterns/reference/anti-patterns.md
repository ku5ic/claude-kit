# Anti-patterns

Severity levels: the report-format skill.

## A runtime value interpolated into a document

`failure`. `` gql`query { user(id: "${id}") { name } }` `` makes the value part of the document. The client builds GraphQL text at runtime and every value is a new document. Declare `$id: ID!` and pass `variables: { id }`.

## An object selected without its identifier field

`warning`. Without `id` (or the type's `keyFields`), Apollo can't normalize the object, so a mutation that returns the updated entity doesn't update the other views showing it. Select the identifier.

## Reading v3's `errors` or `ApolloError` on Apollo Client 4

`failure`. v4 removed both, so `error.graphQLErrors` checks and `if (result.errors)` branches silently never fire. Use `CombinedGraphQLErrors.is(error)` and `error.errors`.

## `errorPolicy: "all"` handled as all-or-nothing

`warning`. With `"all"`, data and errors come back together. Code that shows an error screen whenever `error` is set drops the partial data, and code that reads `data` without checking `error` shows partial data as complete. Handle both.

## `cache-and-network` passed to `client.query`

`failure`. `client.query` returns one result; the policy throws a development invariant. Use `client.watchQuery`, or `cache-first` / `network-only`.

## Hand edits in the codegen output directory

`failure`. The client preset regenerates it from `codegen.ts` and the source documents, so the edit is lost on the next run. Change the document or the config.
