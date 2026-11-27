---
name: mermaid-patterns
description: Mermaid patterns for flowchart, sequence, quadrant, xychart, and pie syntax, config, and multi-chart files. Load before any edit to a .mmd file or a mermaid fence, however small, or for Mermaid.
---

# Mermaid patterns

Default assumption: Mermaid 12.3.

- The syntax rules hold for 11.x too. What 12.0 changed is in [reference/config.md](reference/config.md).
- Adapt advice to the Mermaid the viewer or `mmdc` bundles, per [reference/rendering.md](reference/rendering.md).

## Reference files

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                                                  | Read                                                     |
| ----------------------------------------------------------------------- | -------------------------------------------------------- |
| Writing or changing a flowchart, sequence, quadrant, xychart, or pie    | [reference/syntax.md](reference/syntax.md)               |
| Setting a theme, colors, layout, or any other config on a diagram       | [reference/config.md](reference/config.md)               |
| Putting more than one chart in a file, or checking that a chart renders | [reference/rendering.md](reference/rendering.md)         |
| Writing, changing, or reviewing a Mermaid diagram                       | [reference/anti-patterns.md](reference/anti-patterns.md) |
