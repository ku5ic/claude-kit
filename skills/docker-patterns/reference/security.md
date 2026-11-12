# Security

## Image

- Create a non-root user with an explicit numeric UID and GID, and set `USER` before the final `CMD` or `ENTRYPOINT`.
- Use the smallest base image that meets the runtime's needs: `distroless` (the application and its runtime dependencies only, no shell or package manager), a `-slim` variant of an official image, or `scratch` for a statically compiled binary.
- Production images carry no debugging tools. Put them in a separate `debug` stage and build it with `--target debug`.
- Floating tags (`latest`, `3.12`): pin a version tag such as `python:3.12.10-slim`, or a digest (`@sha256:...`) for full immutability.
- A pinned image: use Docker Scout or a dependency update tool to hear when a CVE fix is out.

## Build context

- `.dockerignore` lists `.env`, `.env.*`, `*.pem`, `*.key`, `*.crt`, `secrets/`, `.git`, `.github`, `node_modules`, and `__pycache__`.
- Never `COPY` a secret into an image, and never pass one through an `ARG` used in `RUN`, which can expose it in layer history. Inject secrets at runtime.

## Runtime secrets

From least to most secure:

1. `docker run -e SECRET=value`, visible in `docker inspect`.
2. `env_file` with restricted file permissions, still visible in `docker inspect`.
3. Docker secrets (Compose or Swarm), mounted as files at `/run/secrets/<name>`, absent from `docker inspect` and from image layers.

- With Docker secrets, the application reads the file at runtime, pointed at it by a variable such as `POSTGRES_PASSWORD_FILE: /run/secrets/db_password`.

## Scanning and hardening

- Run Docker Scout (`docker scout cves <image>`, or Docker Desktop) in CI on every image build. Fix `critical` and `high` CVEs before pushing to production.
- Set `read_only: true` on the service, with `tmpfs:` for scratch paths such as `/tmp` and `/var/run` and a volume for each other path that needs writes, such as uploads.
