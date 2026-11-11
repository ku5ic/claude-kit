# graphql-codegen: the client preset

- `codegen.ts`: export a `CodegenConfig` whose `generates` target sets the preset, as in `generates: { "./src/gql/": { preset: "client" } }`.
- `documents`: exclude the output directory, as in `["src/**/*.tsx", "!src/gql/**/*"]`.
- The output directory: review `codegen.ts` and the source documents, never hand edits in `src/gql/`.
- A document: import `graphql` from the generated directory and wrap the document in it, as in ``graphql(/* GraphQL */ `query ...`)``.
- A `graphql()` document: pass it to the client with no generic arguments and no generated hooks. A hand-written generic on `useQuery<...>` next to one is a finding.
- A component's fragment data, with fragment masking on (the default): type the prop as `FragmentType<typeof ItemFragment>` and unmask it with `useFragment(ItemFragment, props.item)`.
- When the rules-of-hooks lint rule flags `useFragment` for its hook-like name, rename it with `presetConfig.fragmentMasking.unmaskFunctionName: 'getFragmentData'`, never by disabling the lint rule.
- `presetConfig.fragmentMasking: false`: only as a deliberate project decision, never as a fix for a type error.
