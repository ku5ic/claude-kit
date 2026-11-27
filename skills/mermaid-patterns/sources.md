# mermaid-patterns sources

The sources and version notes behind this skill, for `/meta-refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- Mermaid docs: https://mermaid.js.org/
- Changelog: https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/CHANGELOG.md
- Releases: https://github.com/mermaid-js/mermaid/releases
- mermaid-cli releases: https://github.com/mermaid-js/mermaid-cli/releases

### Version notes

Checked: 2026-11-27 against https://api.github.com/repos/mermaid-js/mermaid/releases/tags/mermaid@12.3.0

- 12.3.0 (2026-11-03): current. Removed the legacy ER renderer and the `uuid` dependency.
- 12.0.0 (2026-09-10): ELK the default layout, `redux-color` and `neo` the default theme and look, `defaultRenderer` removed. 11.17.2 (2026-08-25) is the last 11.x.
- mermaid-cli 12.2.0 (2026-11-02) depends on `mermaid ^12.2.0`.
- Reproduced 2026-11-27 with Typora's bundled Mermaid 12.2.0, rendering one file's charts concurrently through `mermaidAPI.render`: shared ids swapped nodes between charts, and a pie's `title` showed in the others.

## reference/syntax.md

### Claims

- Flowchart, a comment: https://mermaid.js.org/syntax/flowchart.html
- Flowchart, a lowercase `end`: https://mermaid.js.org/syntax/flowchart.html
- Flowchart, `A---oB`: https://mermaid.js.org/syntax/flowchart.html
- Flowchart, `linkStyle` takes edge indexes: https://mermaid.js.org/syntax/flowchart.html
- Flowchart, invisible `~~~` links count: https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/diagrams/flowchart/flowDb.ts
- Flowchart, quote a label: https://mermaid.js.org/syntax/flowchart.html
- Flowchart, a subgraph's `direction`: https://mermaid.js.org/syntax/flowchart.html
- sequenceDiagram, a `;` in a message: https://mermaid.js.org/syntax/sequenceDiagram.html
- sequenceDiagram, the word `end`: https://mermaid.js.org/syntax/sequenceDiagram.html
- quadrantChart, a point: https://mermaid.js.org/syntax/quadrantChart.html
- xychart, both parse: https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/diagrams/xychart/xychartDetector.ts
- xychart, series and axes: https://mermaid.js.org/syntax/xyChart.html
- pie, syntax and values: https://mermaid.js.org/syntax/pie.html
- Limits, `maxEdges` and `maxTextSize`: https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/schemas/config.schema.yaml

## reference/config.md

### Claims

- Frontmatter over directives, directives are deprecated: https://mermaid.js.org/config/directives.html
- Frontmatter over directives, frontmatter is a YAML block: https://mermaid.js.org/config/configuration.html
- What 12.0 changed, every line: https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/CHANGELOG.md
- What 12.0 changed, 12.3.0 removed the legacy ER renderer: https://github.com/mermaid-js/mermaid/releases/tag/mermaid@12.3.0
- Security, `securityLevel` defaults to `strict`: https://mermaid.js.org/config/usage.html

## reference/rendering.md

### Claims

- A file whose charts render at once, `mermaid.render` queues calls: https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/mermaid.ts
- A file whose charts render at once, `mermaidAPI.render` has no queue: https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/mermaidAPI.ts
- A file whose charts render at once, a large re-architecture: https://github.com/mermaid-js/mermaid/issues/4369
- A file whose charts render at once, the diagram title: https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/diagrams/common/commonDb.ts
- A file whose charts render at once, the cluster map and a subgraph no edge crosses: https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/rendering-util/layout-algorithms/dagre/mermaid-graphlib.js
- Checking a render, `mermaid.run` names each SVG: https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/utils.ts

## reference/anti-patterns.md

### Claims

- Lowercase `end` in a flowchart label, The flowchart doesn't parse: https://mermaid.js.org/syntax/flowchart.html
- A node id starting with `o` or `x` after `---`, `A---oB` draws: https://mermaid.js.org/syntax/flowchart.html
- A subgraph no edge crosses, A viewer rendering: https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/rendering-util/layout-algorithms/dagre/mermaid-graphlib.js
- The same id in two charts of one file, A viewer rendering: https://github.com/mermaid-js/mermaid/issues/4369
- A `title` in a chart that shares a file, A viewer rendering: https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/diagrams/common/commonDb.ts
- A `%%{init: ...}%%` directive in a new diagram, Deprecated since 10.5.0: https://mermaid.js.org/config/directives.html
