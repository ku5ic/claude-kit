# Story ids and URLs

## How an id is formed

- A story id is the title and the story's export name, each kebab-cased, joined by `--`. Title `Foo/Bar` with export `Baz` gives `foo-bar--baz`.
- The title comes from `title` in the meta. In CSF 3 it can be left out, and Storybook infers it from the file's location.
- An `id` in the meta replaces the title part of the id.
- A story's `name` changes only the display text. `export const Primary = { name: 'Main button' }` is still `...--primary`.

## URLs

- A story in the Storybook UI: `?path=/story/<story-id>`, as in `http://localhost:6006/?path=/story/foo-bar--baz`.
- The story alone, for tools that test one rendered story such as axe: `iframe.html?id=<story-id>&viewMode=story`.

## What breaks links

- Renaming an export, a title, or the directory an auto-title comes from changes the id. MDX `<Story id="..."/>` embeds, bookmarked URLs, and external links to the old id stop resolving.
- An intended rename: search for the old id.
