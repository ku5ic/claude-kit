# Anti-patterns

Severity levels: the report-format skill.

- Reading v3's `errors` array or `ApolloError` on Apollo Client 4, as in `error.graphQLErrors` or `if (result.errors)`: `failure`. Use `CombinedGraphQLErrors.is(error)` and `error.errors`.
- A runtime value interpolated into a document, as in `` gql`query { user(id: "${id}") { name } }` ``: `failure`. Declare `$id: ID!` and pass `variables: { id }`.
- An object selected without its identifier field, `id` or the type's `keyFields`: `warning`. Select the identifier.
- `errorPolicy: "all"` handled as all-or-nothing: `warning`. Show the partial `data` when `error` is set, and check `error` before treating `data` as complete.
- `fetchPolicy: "cache-and-network"` passed to `client.query`: `failure`. Use `client.watchQuery`, or `cache-first` / `network-only`.
- A hand edit in the codegen output directory: `failure`. Change the source document or `codegen.ts`.
