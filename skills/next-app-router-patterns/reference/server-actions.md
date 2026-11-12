# Server Functions and Server Actions

A Server Function is an async function that runs on the server and that the client calls over the network. A Server Action is a Server Function used for a form submission or other mutation, passed to `<form action={fn}>` or `<button formAction={fn}>`; how it's invoked decides the name. React's `startTransition` dispatches a Server Action as a `POST`, automatically when it's passed to a `<form>` or `<button>` action prop.

## Pending state

- On React 19, wrap a Server Action in `useActionState`, and read `pending` in a form's child with `useFormStatus`. The hooks' semantics are react-patterns' `reference/structure.md`.

## `"use server"` directive

- At the top of a file, every export is a Server Function. At the top of an async function, that function is one.
- A Client Component can't define a Server Function; it imports one from a `"use server"` file.
- A form action receives a `FormData` instance; read fields with `formData.get("title")`.

## Authenticate inside every Server Function

- A Server Function without its own auth check: `failure`. Verify authentication and authorization inside the function every time, never only in Proxy or middleware.
- Read auth from cookies or headers via `auth()`, never from function parameters.

## Validate input at the boundary

- Validate every `FormData` field with a schema library (Zod, Valibot) at the top of the function, before the database call.
- `formData.get("x")` returns `FormDataEntryValue | null`; treat a missing or wrong-typed field as user input, not as a bug.

## Mutate then revalidate

- After a mutation, call `revalidatePath()` or `revalidateTag()` from `next/cache`. Under Cache Components, prefer `updateTag()` for `cacheTag`-marked content.
- To navigate after a mutation, call `redirect()` from `next/navigation`. It throws, so nothing after it runs.
- Use `refresh()` from `next/cache` only when the cache is already current and the router needs a soft refresh; it doesn't revalidate tagged data.

## Limit return values

- Return only the fields the UI needs, never raw database records; return values are serialized to the client.
- Constrain return shapes in one place with a Data Access Layer, as the Next.js Data Security guide recommends.
