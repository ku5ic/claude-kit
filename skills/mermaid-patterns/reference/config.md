# Config

## Frontmatter over directives

- Directives (`%%{init: ...}%%`) are deprecated since 10.5.0, in favor of the frontmatter `config:` key. The 12.x docs still document them.
- Frontmatter is a YAML block before the diagram type line. Its config overrides the site config, except the secure keys.

```yaml
---
config:
  theme: base
  themeVariables:
    primaryColor: "#00ff00"
---
```

## What 12.0 changed

- ELK is bundled, and is the default layout for flowchart, state, class, ER, requirement, and use-case diagrams. `layout: dagre` restores the old one. The tiny build falls back to dagre.
- `redux-color` is the default theme and `neo` the default look. `theme: default` with `look: classic` restores the old look.
- `defaultRenderer` is gone from the flowchart, class, and state config. An old setting is ignored, not rejected. Set `layout` instead.
- It requires ES2024, Safari 17.4 or later, and Node.js 22.12 or later.
- 12.3.0 removed the legacy ER renderer.

## Security

- `securityLevel` defaults to `strict`, which encodes HTML tags and disables click handlers. `loose` enables clicks, and `sandbox` renders the diagram in a sandboxed iframe.
