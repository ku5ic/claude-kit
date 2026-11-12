# Apollo Client 4

- React hooks on v4: import them from `@apollo/client/react`, as in `import { useQuery } from "@apollo/client/react"`. A hook imported from the package root is v3 code.
- Errors on v4: read the one `error` property. v3's separate `errors` array and `ApolloError` are gone.
- A GraphQL error on v4: check `CombinedGraphQLErrors.is(error)`, then read `error.errors`. `is` handles a nullish `error`, so add no separate null check.
- `useLazyQuery`'s execute function on v4 with `errorPolicy: "none"`: its promise rejects on GraphQL and network errors. Handle them with try/catch, never by reading `error` off the resolved value.
- `notifyOnNetworkStatusChange` defaults to `true` on v4, so refetches show loading states. A v3 app that relied on the old default sets it back under `defaultOptions.watchQuery`.
- A cache ID inside a link on v4: `operation.client.cache.identify`, never v3's `getContext().getCacheKey`.
- Local `@client` resolvers on v4: `new LocalState({ resolvers })`, passed as the client's `localState` option.
- `errorPolicy: "all"`: data and errors come back together. Handle a `data` that holds some fields while `error` is set.
- Cross-cutting error logging: an `ErrorLink` from `@apollo/client/link/error`, placed before the terminating `HttpLink`.
- `client.query`: never with `fetchPolicy: "cache-and-network"`, which throws a development-only invariant. Use `client.watchQuery` for cache-then-network, or `cache-first` / `network-only`.
- A type whose identifier isn't `id`: name its fields in `typePolicies: { Type: { keyFields: [...] } }`. The cache keys objects by `__typename` plus those fields.
- A type for transient data that never updates, like metrics: `keyFields: false` stops normalizing it.
- An object's cache ID: `cache.identify(obj)`, never a hand-built `Type:id` string.
- A non-normalized nested object that incoming writes would replace: a `merge` function in its field policy, using the `mergeObjects` helper.
