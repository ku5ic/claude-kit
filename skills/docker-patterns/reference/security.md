# Security

## Contents

- [Non-root user](#non-root-user)
- [Minimal base images](#minimal-base-images)
- [Pin base images](#pin-base-images)
- [.dockerignore](#dockerignore)
- [Secrets management](#secrets-management)
- [Docker Scout](#docker-scout)
- [Read-only filesystem](#read-only-filesystem)
- [References](#references)

## Non-root user

Run the application as a non-root user. By default, Docker containers run as root inside the container, which is a significant risk if the container is ever escaped.

```dockerfile
RUN groupadd --gid 1001 appgroup \
  && useradd --uid 1001 --gid appgroup --no-create-home appuser
USER appuser
```

Set `USER` before the final `CMD` or `ENTRYPOINT`. Assign explicit numeric UID/GID so volume ownership is predictable and consistent across host systems.

## Minimal base images

Each package in a base image is a potential attack surface. Use the smallest image that satisfies the application's runtime requirements:

- `distroless` images contain only the application and its runtime dependencies. No shell, no package manager.
- `-slim` variants of official images remove most optional packages.
- `scratch` is appropriate for statically compiled binaries.

Do not install debugging tools in production images. If you need to debug, use `--target debug` with a separate Dockerfile stage that includes debugging tools.

## Pin base images

Floating tags (`latest`, `3.12`) allow a `docker pull` to silently change the base. Pin to a specific version tag in Dockerfiles, and optionally to a digest for complete immutability:

```dockerfile
# Version pinned (tag can still be overwritten by the registry)
FROM python:3.12.10-slim

# Digest pinned (immutable; use Docker Scout to get updates)
FROM python:3.12.10-slim@sha256:abc123...
```

Use Docker Scout or a dependency update tool to get notified when a pinned image has a CVE fix available.

## .dockerignore

Prevent secrets and unnecessary files from entering the build context:

```
.env
.env.*
*.pem
*.key
*.crt
secrets/
.git
.github
node_modules
__pycache__
```

A file that reaches the build context can be extracted from the image layers even if it is not explicitly `COPY`-ed; ARG values used in `RUN` instructions can be exposed in layer history. Never `COPY` secrets; inject them at runtime.

## Secrets management

Never bake secrets into images. Three patterns in order of increasing security:

1. Environment variables at runtime: `docker run -e SECRET=value`. Visible in `docker inspect`.
2. `env_file` with restricted file permissions. Still visible in inspect.
3. Docker secrets (Compose or Swarm): mounted as files at `/run/secrets/<name>`. Not visible in inspect output, not in image layers.

```yaml
secrets:
  db_password:
    file: ./secrets/db_password.txt

services:
  db:
    image: postgres:16
    secrets:
      - db_password
    environment:
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
```

The application reads the secret from the file at runtime. The password is never in an environment variable or image layer.

## Docker Scout

Docker Scout analyzes images for known CVEs by generating a Software Bill of Materials (SBOM) and cross-referencing it against a vulnerability database. Available via `docker scout cves <image>` or Docker Desktop.

Run Scout as part of CI on every image build. Address `critical` and `high` CVEs before pushing to production.

## Read-only filesystem

Mount the root filesystem read-only and provide writable volumes only where needed:

```yaml
services:
  api:
    image: myapi
    read_only: true
    tmpfs:
      - /tmp
      - /var/run
    volumes:
      - uploads:/app/uploads
```

A read-only root filesystem limits the blast radius of a container escape or RCE.

## References

- https://docs.docker.com/build/building/best-practices/
- https://docs.docker.com/scout/
