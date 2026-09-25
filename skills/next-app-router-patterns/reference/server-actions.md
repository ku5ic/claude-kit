# Server Functions and Server Actions

In Next.js 16, **Server Function** is the broader term: an async function that runs on the server, callable from the client through a network request. **Server Action** is the narrower term: a Server Function used for form submissions or other mutations (passed to `<form action={fn}>` or `<button formAction={fn}>`). The same function is both: how it is invoked decides which name applies.

Server Actions are dispatched as `POST` requests by React's `startTransition`; this happens automatically when the function is passed to a `<form>` or `<button>` action prop.

## `"use server"` directive

Two placements:

- File-top: every export from the file is a Server Function.
- Function-top: the single async function is a Server Function.

```ts
"use server";
import { auth } from "@/lib/auth";

export async function createPost(formData: FormData) {
  const session = await auth();
  if (!session?.user) throw new Error("Unauthorized");
  // mutate
}
```

A Client Component cannot define a Server Function; it imports one from a `"use server"` file and invokes it.

## Invoking from a form

```tsx
import { createPost } from "@/app/actions";

export function CreatePostForm() {
  return (
    <form action={createPost}>
      <input name="title" required />
      <button type="submit">Create</button>
    </form>
  );
}
```

The action receives a `FormData` instance. Use `formData.get("title")` to read fields.

## Authenticate inside every Server Function

`failure`. Server Functions are reachable as direct `POST` requests, not just through your app's UI. The Next.js docs call this out explicitly: do not rely on Proxy / middleware to gate access; verify auth and authorization inside the function itself, every time. Read auth from cookies or headers via `auth()`, not from function parameters.

## Validate input at the boundary

Server Functions accept arbitrary `FormData` from the network. Validate every field with a schema library (Zod, Valibot) at the top of the function, before the database call. `formData.get("x")` returns `FormDataEntryValue | null`; treat absence and wrong-type as user input, not as a bug.

## Mutate then revalidate

After a mutation, call `revalidatePath()` or `revalidateTag()` from `next/cache` so the next read serves fresh data. With Cache Components, prefer `updateTag()` for `cacheTag`-marked content. To navigate after the mutation, call `redirect()` from `next/navigation` (it throws a control-flow exception; nothing after it executes).

```ts
"use server";
import { revalidateTag } from "next/cache";
import { redirect } from "next/navigation";

export async function publishPost(id: string) {
  // auth + validate
  // mutate
  revalidateTag("posts");
  redirect(`/posts/${id}`);
}
```

`refresh()` from `next/cache` refreshes the client router but does not revalidate tagged data; reach for it only when the cache is already up-to-date and only a soft router refresh is needed.

## Pending state: `useActionState`, `useFormStatus`

React 19 provides `useActionState(fn, initialState)` (returns `[state, action, pending]`) for wrapping a Server Action with state and pending tracking, and `useFormStatus()` (read inside a form's child Client Component) for reading `pending` from the parent `<form>` without prop drilling. Use them for submit-button disabling, optimistic UI, and inline validation feedback.

## Limit return values

Server Function return values are serialized and sent to the client. Return only what the UI needs. Never return raw database records; cherry-pick fields. The Next.js Data Security guide recommends a Data Access Layer that constrains return shapes in one place.

## References

- Next.js Mutating Data: https://nextjs.org/docs/app/getting-started/mutating-data
- `use server` directive: https://nextjs.org/docs/app/api-reference/directives/use-server
- React Server Functions: https://react.dev/reference/rsc/server-functions
- `revalidatePath`: https://nextjs.org/docs/app/api-reference/functions/revalidatePath
- `revalidateTag`: https://nextjs.org/docs/app/api-reference/functions/revalidateTag
- `redirect`: https://nextjs.org/docs/app/api-reference/functions/redirect
