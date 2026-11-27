# Syntax

## Flowchart

- A comment is a line of its own, starting with `%%`.
- A lowercase `end` in a node label breaks the flowchart. Capitalize the word, or any letter in it.
- `A---oB` draws a circle edge and `A---xB` a cross edge. A node id starting with `o` or `x` right after `---` takes a space before it, or a capital.
- `linkStyle` takes edge indexes in definition order, from 0, as in `linkStyle 1,2,7 color:blue;`. Invisible `~~~` links count.
- Quote a label holding punctuation or unicode text: `id["..."]`. Entity codes are base 10, so `#35;` is `#`.
- A subgraph's `direction` is ignored when any of its nodes links outside it. It takes the parent's direction instead.

## sequenceDiagram

- A `;` in a message needs `#59;`, since a semicolon can stand for a line break. A `#` needs `#35;`.
- The word `end` in a message can break the diagram. Wrap it in `()`, `""`, `{}`, or `[]`.

## quadrantChart

- A point is `label: [x, y]`, each from 0 to 1.
- quadrant-1 is top right, quadrant-2 top left, quadrant-3 bottom left, and quadrant-4 bottom right.

## xychart

- `xychart` and `xychart-beta` both parse.
- Series are `bar [...]` and `line [...]`. A quoted name before the list, `bar "name" [...]`, adds the series to the legend, from 11.17.0.
- The y-axis is numeric only: `y-axis title min --> max`. The x-axis takes a range or a category list, `x-axis "title" [a, "b c", d]`.

## pie

- `pie [showData] [title text]`, then one `"label" : value` line per slice.
- Values are numbers above 0, with up to two decimal places. A negative value is an error.

## Limits

- `maxEdges` defaults to 500, and `maxTextSize` to 50000 characters.
