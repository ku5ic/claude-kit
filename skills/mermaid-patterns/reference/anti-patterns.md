# Anti-patterns

<!-- chat: skip -->

Severity rubric: `skills/report-format`.

<!-- chat: end -->

## Lowercase `end` in a flowchart label

`failure`. The flowchart doesn't parse. Write `End`, or quote the label.

## A node id starting with `o` or `x` after `---`

`warning`. `A---oB` draws a circle edge to `B`, not a line to `oB`. Put a space before the id, or capitalize it.

## A subgraph no edge crosses, in a file with other charts

`warning`. A viewer rendering the charts together fails the layout with "undefined is not an object (evaluating '….get(E.id).node=E')". Draw the grouping without a subgraph, or join it with an edge that crosses its border ([rendering.md](rendering.md)).

## The same id in two charts of one file

`warning`. A viewer rendering the charts together swaps their nodes. Prefix each chart's ids with its own short name.

## A `title` in a chart that shares a file

`warning`. A viewer rendering the charts together shows that title in the others. Name the chart in the heading above it.

## An `mmdc` pass taken as proof a file renders

`warning`. `mmdc` renders each chart alone, so it misses what breaks when a viewer renders them together, and its Mermaid may differ from the viewer's. Check the rules in [rendering.md](rendering.md) by reading the file.

## A `%%{init: ...}%%` directive in a new diagram

`info`. Deprecated since 10.5.0. Put the settings under the frontmatter `config:` key ([config.md](config.md)).
