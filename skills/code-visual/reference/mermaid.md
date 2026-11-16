# Mermaid limits for the diagrams drawn here

- Directives (`%%{init: ...}%%`) are deprecated since Mermaid 10.5.0, in favor of a frontmatter `config:` key. They still work, and when several set the same key, the last wins.
- `maxEdges` defaults to 500 and `maxTextSize` to 50000 characters. A diagram past either fails to render.
- A comment is a line of its own starting with `%%`.
- A lowercase `end` in a flowchart label or a sequence message breaks the parse. Write `End`, or quote it.
- An id starting with `o` or `x` right after `---` becomes a circle or cross edge. Add a space, or capitalize it.
- `linkStyle` indexes count edges in definition order from 0.
- Quote a label holding parentheses or other punctuation: `id["..."]`.
- quadrantChart: a point is `label: [x, y]`, each from 0 to 1. quadrant-1 is top right, 2 top left, 3 bottom left, 4 bottom right.
- xychart: the y-axis is numeric only, series are `bar [...]` and `line [...]`. The installed version takes `xychart` or the older `xychart-beta`.
- pie: quoted labels, values above 0 with at most two decimals, slices clockwise in listed order.
- sequenceDiagram: write `#59;` for a `;` in a message, and `#35;` for a `#`.
