# Apollo Client 4

Sources: the v4 migration guide, https://github.com/apollographql/apollo-client/blob/main/docs/source/migrating/apollo-client-4-migration.mdx, and the docs under https://github.com/apollographql/apollo-client/tree/main/docs/source (Context7 /apollographql/apollo-client).

## Entry points

React hooks come from `@apollo/client/react`, not the package root: `import { useQuery } from "@apollo/client/react"`. Root imports of hooks are v3 code. Source: migration guide.

## Errors

- v4 has one `error` property. v3's separate `errors` array (set only with `errorPolicy: "all"`) and `ApolloError` are gone. Source: migration guide, "Unification of the error property" and "Removal of ApolloError".
- GraphQL errors arrive as a `CombinedGraphQLErrors`: check with `CombinedGraphQLErrors.is(error)`, then read `error.errors`. It handles a nullish `error`, so no separate null check. Source: migration guide; `docs/source/data/error-handling.mdx`.
- With `errorPolicy: "none"`, the promise from `useLazyQuery`'s execute function rejects on GraphQL and network errors: handle it with try/catch, not by reading `error` off the resolved value. Source: migration guide.
- `errorPolicy: "all"` returns data and errors together for partial success. Code using it must handle both: `data` may hold some fields while `error` is set. Source: `docs/agent-skills/apollo-client/references/mutations.md`.
- Cross-cutting logging belongs in an `ErrorLink` from `@apollo/client/link/error`, placed before the terminating `HttpLink`. Source: `docs/source/data/error-handling.mdx`.

## Fetch policies

`client.query` returns one result, so `fetchPolicy: "cache-and-network"` doesn't work with it; a development-only invariant throws. Use `client.watchQuery` for cache-then-network, or `cache-first` / `network-only`. Source: `src/core/ApolloClient.ts`.

## Loading state on refetch

v4 defaults `notifyOnNetworkStatusChange` to `true`, so refetches show loading states. A v3 app that relied on the old behavior sets it back under `defaultOptions.watchQuery`. Source: migration guide.

## Cache

- Normalization keys objects by `__typename` plus identifier fields, `id` by default. Customize with `typePolicies: { Type: { keyFields: [...] } }`; `keyFields: false` stops normalizing a type, for transient data like metrics that never updates. Source: `docs/source/caching/cache-configuration.mdx`.
- Get an object's cache ID with `cache.identify(obj)` rather than building the `Type:id` string by hand. In links, v3's `getContext().getCacheKey` became `operation.client.cache.identify`. Source: `docs/source/caching/cache-interaction.mdx`; migration guide.
- A non-normalized nested object needs a `merge` function in its field policy (the `mergeObjects` helper) so incoming writes don't replace it. Source: `docs/source/caching/cache-field-behavior.mdx`.

## Local state

Local `@client` resolvers go in `new LocalState({ resolvers })`, passed as the client's `localState` option. Source: `docs/source/local-state/local-resolvers.mdx`.
