# Anti-patterns

Severity levels: the report-format skill.

- A secret in `ENV` or `ARG`, including an `ARG` used in `RUN`: `failure`. Inject it at runtime through Docker secrets or a secrets manager.
- A Dockerfile with no `USER` instruction, so the process runs as root: `failure`. Add a non-root `USER`.
- `condition: service_healthy` on a dependency with no `healthcheck`: `failure`. Add a `healthcheck` to the dependency, or use `condition: service_started`.
- A database port published on all interfaces, as in `"5432:5432"`: `failure`. Don't publish internal service ports; services on the same Compose network connect without them.
- `apt-get update` and `apt-get install` in separate `RUN` instructions: `warning`. Combine them in one `RUN`.
- `COPY . .` before the dependency install: `warning`. Copy the manifest and install first, then `COPY . .`.
- A `latest` tag in production: `warning`. Pin a specific version tag.
- No `.dockerignore`: `warning`. Add one that keeps `.git`, `node_modules`, `.env`, and test fixtures out of the build context.
- A single-stage build for a compiled or bundled app (Go, Rust, TypeScript): `info`. Use a multi-stage build with a minimal runtime image.
- Shell form `CMD`, as in `CMD python app.py`: `info`. Use exec form, `CMD ["python", "app.py"]`.
