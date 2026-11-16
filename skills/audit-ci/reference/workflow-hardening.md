# Workflow hardening

- Set `permissions:` at the top or per job. Listing any scope sets every unlisted one to `none`.
- With no `permissions:`, the token gets the org or repo default, which the file can't show. The finding says it's unknown from the file.
- `pull_request_target` runs in the base repo's context with its secrets. Checking out or running the PR's code there is the exploit.
- `workflow_run` runs privileged even when the workflow that triggered it didn't. Its downloaded artifacts are untrusted, so never run them or unpack them into a trusted workspace.
- A value interpolated into `run:` is expanded before the shell parses it. Pass it through `env:` and quote the variable.
- Attacker-shaped values: titles, bodies, comment and review text, commit messages, author names and emails, and branch names (`head_ref`, `head.ref`, `head.label`). A SHA, a number, a boolean, and a function returning a boolean are safe.
- A self-hosted runner on a public repo runs whatever a PR's workflow says, and can keep state between jobs.
- A secret that reached a log needs the log deleted and the secret rotated. A structured secret, such as a JSON blob, can't be masked reliably.
