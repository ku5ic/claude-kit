# Reading a design

- Read in this order, by whichever tools the connector has. An outline of the structure (cheap, to pick nodes), a screenshot, the styling data (variables, styles, generated code), the mapping from design components to code components, then a library search.
- Figma's server names these `get_metadata`, `get_screenshot`, `get_design_context`, `get_variable_defs`, `get_code_connect_map`, and `search_design_system`.
- Generated code from the design tool is a hint for structure and values, never the answer.
- The variables and styles a selection uses (colors, spacing, typography) are its tokens. A value with no variable behind it is hard-coded in the design, and the report says so.
- A design-to-code mapping (Figma's Code Connect) links design components to repo components. A component with no mapping isn't mapped, which isn't the same as missing from the code.
- A variable mode (theme, density, brand) the design shows once implies the others.
- A frame's annotations and ready-for-dev status, when the connector exposes them, go in the report.
- A call that fails or returns a cut-off frame is named in the report, never filled from memory.
