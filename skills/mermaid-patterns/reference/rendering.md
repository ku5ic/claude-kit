# Charts that render in every viewer

## A file whose charts render at once

- `mermaid.render` queues calls and runs them one at a time. `mermaidAPI.render` has no queue, and Mermaid's maintainers call making every diagram safe to render concurrently a large re-architecture. Typora, on Mermaid 12.2.0, calls `mermaidAPI.render` for each fence.
- Concurrent renders share module-level state: the diagram title, and the dagre layout's cluster map. So in one file with more than one chart:
  - No node or subgraph id repeats across charts. Charts rendering together with a shared id swap nodes.
  - No chart sets a `title`, in its first line or its frontmatter. The heading above it names it, since a title shows up in the other charts.
  - Every subgraph has an edge crossing its border. Dagre lays out a subgraph no edge crosses as a graph of its own, through the shared cluster map, and a concurrent render fails with "undefined is not an object (evaluating '….get(E.id).node=E')". A grouping no edge crosses is drawn without a subgraph.

## Checking a render

- `mmdc` renders one chart per page, so its pass says each chart parses and lays out alone. It says nothing about a viewer rendering them together.
- `mmdc --version` prints the CLI's version, not Mermaid's. The Mermaid it renders with is the `version` in its `node_modules/mermaid/package.json`. Compare it to the viewer's before trusting a render.
- `mermaid.run` names each SVG `mermaid-` plus `Date.now()` unless `deterministicIds` is set, so two charts it renders in the same millisecond share an id.
