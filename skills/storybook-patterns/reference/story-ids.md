# Story ids and URLs

## How an id is formed

Storybook builds a story's id from the component title and the story's export name, each kebab-cased, joined by `--`. Title `Foo/Bar` with export `Baz` gives `foo-bar--baz`. Source: https://storybook.js.org/docs/configure/user-interface/sidebar-and-urls

- The title comes from `title` in the meta. In CSF 3 it can be left out, and Storybook infers it from the file's location.
- An `id` in the meta replaces the title part of the id.
- A story's `name` changes only the display text. The id still comes from the export key, so `export const Primary = { name: 'Main button' }` is still `...--primary`.

Source: https://storybook.js.org/docs/configure/user-interface/sidebar-and-urls

## URLs

- In the Storybook UI: `?path=/story/<story-id>`, e.g. `http://localhost:6006/?path=/story/foo-bar--baz`. Source: https://storybook.js.org/docs/configure/user-interface/sidebar-and-urls
- The story alone, without the UI around it: `iframe.html?id=<story-id>&viewMode=story`. This is the URL to hand to tools that test a single rendered story, such as `a11y-check.sh`. Source: https://storybook.js.org/docs/sharing/embed

## What breaks links

Renaming an export, a title, or the directory an auto-title comes from changes the id. Anything pointing at the old id stops resolving: MDX `<Story id="..."/>` embeds, bookmarked URLs, external links. When a rename is intended, search for the old id. `<Story id>` embeds: https://github.com/storybookjs/storybook/blob/next/code/addons/docs/docs/mdx.md
