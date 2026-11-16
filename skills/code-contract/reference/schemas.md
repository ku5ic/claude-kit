# Schema checkers

## OpenAPI, oasdiff

- `oasdiff breaking <base-spec> <head-spec>` reports errors, which are definite breaks, and warnings, which are potential ones. An error is major, a warning a review item.
- A graceful deprecation raises no error, so it's minor.

## GraphQL, graphql-inspector

- `graphql-inspector diff <old-schema> <new-schema>` classes each change breaking, dangerous, or safe. Breaking is major, dangerous a review item.
- A new type or field isn't a break, since a client gets only what it asks for.
- A field marked `@deprecated` stays legal in a query, so the deprecation is minor.

## Protobuf, buf

- `buf breaking --against '.git#branch=<base>'`. The rule set comes from `buf.yaml` `breaking.use`, and the report names it.
- FILE is the strictest set, and a change that passes FILE also passes PACKAGE, WIRE_JSON, and WIRE. Wire-only consumers need WIRE, JSON clients WIRE_JSON, generated-code consumers FILE or PACKAGE.
- A field number is never reused, and a deleted field's number gets reserved.
- Changing a field's number, or moving a field into an existing `oneof`, isn't safe on the wire.
