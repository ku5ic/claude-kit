# graphql-codegen: the client preset

Source for this file: https://github.com/dotansimha/graphql-code-generator/blob/master/website/src/pages/plugins/presets/preset-client.mdx (Context7 /dotansimha/graphql-code-generator).

## Config

`codegen.ts` exports a `CodegenConfig` whose `generates` target uses `preset: 'client'`:

```ts
import type { CodegenConfig } from '@graphql-codegen/cli'

const config: CodegenConfig = {
  schema: 'schema.graphql',
  documents: ['src/**/*.tsx', '!src/gql/**/*'],
  generates: {
    './src/gql/': { preset: 'client' },
  },
}

export default config
```

- `documents` excludes the output directory, so generated files are never re-scanned as sources.
- The output directory is generated code: review `codegen.ts` and the source documents, never hand edits in `src/gql/`.

## Typed documents through `graphql()`

Import `graphql` from the generated directory and wrap each document in it: `graphql(/* GraphQL */ \`query ...\`)`. The result is a typed document node, so the client infers the result and variables types with no generic arguments or generated hooks. A hand-written generic on `useQuery<...>` next to a `graphql()` document is redundant and can drift.

## Fragment masking

On by default. A component takes `FragmentType<typeof ItemFragment>` as its prop and unmasks it with `useFragment(ItemFragment, props.item)`; the parent only sees the fragment reference, so it can't depend on fields it didn't select itself.

- When the rules-of-hooks lint rule flags `useFragment` for its hook-like name, rename it with `presetConfig.fragmentMasking.unmaskFunctionName: 'getFragmentData'` instead of disabling the lint rule.
- `presetConfig.fragmentMasking: false` turns masking off. Treat it as a deliberate project decision, not a fix for a type error.
