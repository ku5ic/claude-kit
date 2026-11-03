# Design slop

Visual and interface output: React and JSX, Vue, HTML, CSS, Tailwind, and the copy inside them. Prose tells are in [content-and-grammar.md](content-and-grammar.md); accessibility is the `wcag-audit` skill.

## 1. What design slop is

A visual decision nobody made. Given no constraint, a model fills each free axis (font, palette, layout, radius, motion) with the most common value it has seen. Each choice is defensible alone; together they give every subject the same page.

The test is never "is it ugly". It's: **would this choice change if the product, audience, or brief changed?** If not, it's a default, and the default is the tell.

Slop comes in waves. Each look that gets called out spawns the next default:

| Wave | Look                                                                                                    |
| ---- | ------------------------------------------------------------------------------------------------------- |
| 1    | Purple or indigo gradient, Inter, centered hero, three rounded icon cards, soft shadows                 |
| 2    | Warm cream background (near `#F4F1EA`), high-contrast serif display, terracotta accent (near `#D97757`) |
| 2    | Near-black background with one acid-green or vermilion accent                                           |
| 2    | Broadsheet: hairline rules, zero radius, dense newspaper columns                                        |

Wave 2 is what a model produces when told to look less generated. Banning wave 1 just moves the default. Target unchosen defaults, not a color list.

## 2. The brief wins

Every pattern below is fine when the brief, the brand, or the project's design system asks for it. Before flagging:

1. Check the brief. If it names the look, build that look, even one listed here.
2. Check the project's tokens: Tailwind v4 `@theme`, `tailwind.config.*`, CSS custom properties, a `DESIGN.md`, a design-system package.
3. A value from those tokens is a decision. A raw default (`indigo-500`, `font-sans` resolving to Inter, `rounded-2xl` on everything) in a project with its own tokens is the tell.

## 3. Color

| Tell                                                                        | Instead                                                                                                  |
| --------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| Purple, indigo, or violet gradient on the hero, headline, or primary button | A palette derived from the subject: 4 to 6 named colors with roles (base, surface, text, accent, signal) |
| Gradient text on headings or big numbers                                    | Solid color; emphasis from size, weight, or position. Gradient text also fails contrast easily           |
| Neon or cyan accents on dark, glowing borders                               | One accent with one job, at full contrast                                                                |
| Radial glow or blurred blobs behind content                                 | Remove; depth comes from surface levels                                                                  |
| Five or six saturated colors at equal weight                                | One dominant, one accent, one neutral; decide what earns the accent                                      |
| Cream surface by reflex                                                     | Fine when the subject calls for it                                                                       |
| A tinted near-black (`#0B0B0B`, `#111`) everywhere                          | The palette's own text color                                                                             |

## 4. Typography

| Tell                                                           | Instead                                                                                                 |
| -------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| Inter, Geist, Roboto, or the system stack as the display face  | A face chosen for the subject. Inter can stay for body text if a distinct display face carries identity |
| One word in the headline italic, bold, or colored              | The whole headline carries the treatment, or none of it                                                 |
| A tracked-out uppercase eyebrow over every heading             | Drop it, or keep it where it names a real category                                                      |
| A pill badge above the hero headline ("New", "Introducing v2") | The news in the headline, or nothing                                                                    |
| A headline that fills the first screen                         | A size from a type scale                                                                                |
| Italic serif display as an editorial shortcut                  | Only for editorial subjects                                                                             |
| Headings and body nearly the same size or weight               | A scale with clear steps                                                                                |
| Monospace for small labels by reflex                           | Only for code, ids, tabular figures                                                                     |
| Lines past about 80 characters                                 | `max-w-prose` or a `ch` max width                                                                       |

## 5. Layout

| Tell                                                                    | Instead                                                                                         |
| ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| Centered hero: headline, subline, two buttons                           | Open with the most characteristic thing in the subject's world: a demo, real data, one sentence |
| Three or six identical icon-heading-text cards                          | Rank the points; give the important one room; drop the weak ones                                |
| Hero metric block: big number, small label, three stats                 | The number in context, or none                                                                  |
| Bento grid with no reason for its cell sizes                            | Cells sized by content, or a plain flow                                                         |
| Cards inside cards                                                      | Whitespace and proximity; cards only for independently actionable items                         |
| `01 / 02 / 03` markers on things that aren't a sequence                 | Numbers only for steps, timelines, rankings                                                     |
| One gap size everywhere                                                 | Tight inside groups, wide between them                                                          |
| Hero, Logos, Features, Testimonials, Pricing, FAQ, CTA, for any product | Order by what this audience needs first                                                         |

## 6. Surfaces and effects

| Tell                                                         | Instead                                                     |
| ------------------------------------------------------------ | ----------------------------------------------------------- |
| One large radius on cards, buttons, inputs, and images alike | A radius scale tied to element size and role                |
| The same soft shadow under every card                        | Elevation only where something floats: menus, dialogs, drag |
| Hairline border and diffuse shadow on one surface            | An edge or an elevation, not both                           |
| Glass panels and frosted cards by reflex                     | Only where layering is real, like a sheet over a map        |
| A thick colored left border on a plain card                  | Side stripes for status only: alert, quote                  |
| Grid, dot, or noise backgrounds as decoration                | A plain surface unless the texture means something          |
| Gradient washes between sections                             | Spacing and a type change                                   |

## 7. Motion

| Tell                                                 | Instead                                              |
| ---------------------------------------------------- | ---------------------------------------------------- |
| Fade-and-slide-up on every section                   | One orchestrated moment, or none                     |
| Hover lift on every card                             | Hover feedback only on things you can act on         |
| Bounce easing on routine actions                     | Standard easing                                      |
| A pulsing dot on state that never changes            | A dot tied to a real state, with a text label        |
| A scrolling logo marquee                             | A static row, or nothing                             |
| Any motion without `prefers-reduced-motion` handling | Always handle it; it's also an accessibility failure |

## 8. Icons and imagery

| Tell                                                          | Instead                                                             |
| ------------------------------------------------------------- | ------------------------------------------------------------------- |
| Emoji as UI icons in nav, headings, buttons, cards            | One icon set; emoji only in copy, for tone                          |
| An icon in a tinted rounded square over every feature heading | The icon inline with its label, or none                             |
| Blob and abstract-circle hero art                             | Real screenshots, photos, or nothing                                |
| Placeholder images or an empty `src`                          | Real assets, or the layout without the slot                         |
| Icons with no accessible name                                 | `aria-hidden` when decorative, a label when the icon is the content |

## 9. Template chrome

Small pieces that show up whatever the subject:

- Meta strings joined with a middle dot (U+00B7).
- Labels shaped `WORD - fragment`, or the same with an em dash (U+2014).
- A right arrow (U+2192) or `->` after every link and button label.
- "Live", "Beta", "New" badges with nothing live, beta, or new behind them.
- Status dots and colored tags on nav items.
- A "Trusted by" row of placeholder logos.

## 10. Copy in the UI

Interface copy follows [content-and-grammar.md](content-and-grammar.md). The UI-specific tells:

| Tell                                                                              | Instead                                            |
| --------------------------------------------------------------------------------- | -------------------------------------------------- |
| Marketing verbs: supercharge, empower, unlock, world-class, enterprise-grade      | The literal verb and noun of what the product does |
| Hero lines: "Build faster with AI", "X, reimagined", "The future of Y"            | One sentence a user would repeat to a colleague    |
| Slogan contrasts: "Not just a tool. A platform."                                  | What it is                                         |
| Fragment triplets: "Simple. Fast. Secure."                                        | One concrete claim with a fact behind it           |
| Generic buttons: "Get started", "Learn more", "Submit"                            | What happens: "Create workspace", "Save changes"   |
| One action named differently across a flow ("Publish", then "Your post is live!") | One word throughout: "Publish", "Published"        |
| "Oops! Something went wrong"                                                      | What happened and how to fix it                    |
| Mood copy in empty states: "Nothing to see here yet!"                             | The next action                                    |
| Title Case Buttons                                                                | Sentence case                                      |
| Lorem ipsum, "Feature one"                                                        | Real content from the brief                        |

## 11. What the scan catches

`kit slop` reads UI files (`.tsx`, `.jsx`, `.vue`, `.svelte`, `.html`, `.css`, `.scss`) line by line with the `on: [ui]` rules in `kit.yml`: `ui-placeholder`, `ui-emoji-label`, `ui-default-gradient`, `ui-gradient-text`, `ui-marketing`, and the tier 3 `ui-glass`, `ui-eyebrow`, `ui-chrome-glyph`.

It can't see layout (identical card grids, hero metric blocks, section order, nested cards), motion without reduced-motion handling (that needs the whole project), or anything a token resolves at build time. Those belong to the review in section 12. A false block on a correct design costs more trust than a missed gradient, so the scan errs toward silence.

## 12. Review

For `/unslop` on UI work, or before calling a UI change done:

1. Read the brief and the project's tokens (section 2). List the axes the brief fixes and the ones it leaves free.
2. Run `kit slop` on the changed files; fix tier 1 and 2 hits.
3. Render the page and screenshot it at desktop and mobile widths. Judge the screenshot, not the code.
4. Walk sections 3 to 9 against the screenshot. For each hit, ask the section 1 question; replace the default with a choice tied to the subject, or remove it.
5. Spend boldness in one place. One element is memorable; the rest is quiet. Remove one more decoration than feels necessary.
6. Copy pass with section 10.
7. Quality floor: visible keyboard focus, WCAG AA contrast (gradient text and gray on color fail most), reduced motion, ordered headings, line length, layout down to 320px. The full check is the `wcag-audit` skill.

Don't fix wave 1 by reaching for wave 2. Purple to cream is the same mistake.

## 13. False positives

- The brand really is purple: use it from tokens, not Tailwind's defaults.
- Inter as body text in a dense product UI is a fine choice; the tell is Inter as the only identity.
- Cards suit independently actionable items: a repo list, a kanban board.
- A dense dashboard repeats radius and shadow; that's consistency, if the values come from tokens.
- Marketing pages use marketing language. Flag it when it's vague, not when it exists.
- A design system or component library defines the defaults on purpose. Scan its docs and stories; don't flag its token definitions or its variant names (an `elevated` card).
