# Token sources

- A design tokens file (`.tokens` or `.tokens.json`) follows the DTCG format. An object with a `$value` is a token, `$type` sits on it or its group, and an alias is `{group.token}`. Older files use `value` and `type` without the `$`, so match both.
- Tools shouldn't infer a token's type from its group's name.
- Style Dictionary reads its config from `config.json` or `config.js` in the package root, or `--config`. Its `source` globs are the definitions. The files under each platform's `buildPath` are generated, never audited as definitions.
- Tailwind v4 theme variables are CSS variables defined with `@theme`, and they decide which utility classes exist. Variables in `:root` make no utility. The tailwind-patterns skill holds the rest.
- A CSS custom property's name is case-sensitive and scoped to its selector. One name set under `:root` and again under a theme selector is one token with two values, one per mode.
- Also look in `tailwind.config.*` (v3 `theme.extend`), theme objects in code, and a design tool's variable export.
