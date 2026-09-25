# Dockerfile

## Contents

- [Base image selection](#base-image-selection)
- [Layer caching](#layer-caching)
- [RUN instructions](#run-instructions)
- [COPY vs ADD](#copy-vs-add)
- [CMD and ENTRYPOINT](#cmd-and-entrypoint)
- [USER and non-root execution](#user-and-non-root-execution)
- [WORKDIR](#workdir)
- [.dockerignore](#dockerignore)
- [References](#references)

## Base image selection

Choose base images from trusted sources: Docker Official Images, Verified Publisher images. Smaller base images reduce download time, image size, and attack surface.

| Base                   | Size    | Use case                                         |
| ---------------------- | ------- | ------------------------------------------------ |
| `alpine:3`             | ~6 MB   | General purpose; musl libc (check compatibility) |
| `debian:bookworm-slim` | ~75 MB  | When glibc is required                           |
| `python:3.12-slim`     | ~130 MB | Python apps with fewer system packages           |
| `distroless/python3`   | ~50 MB  | Production: no shell, no package manager         |
| `scratch`              | 0       | Statically compiled binaries only (Go, Rust)     |

Avoid `latest` tag in production Dockerfiles. Pin to a specific version or digest.

## Layer caching

Docker caches each instruction. A cache miss invalidates all subsequent layers. Order instructions from least to most frequently changed:

```dockerfile
FROM python:3.12-slim

# 1. System deps (rarely change)
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq-dev \
  && rm -rf /var/lib/apt/lists/*

# 2. Python deps (change when requirements.txt changes)
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# 3. Application code (changes frequently)
COPY . .
```

Copying `requirements.txt` before the full `COPY . .` ensures pip install is cached as long as requirements do not change.

## RUN instructions

Combine related `apt-get` commands to avoid stale cache and reduce layers:

```dockerfile
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    libpq-dev \
  && rm -rf /var/lib/apt/lists/*
```

- Always combine `update` and `install` in the same `RUN`.
- `--no-install-recommends` avoids pulling in optional packages.
- Remove `/var/lib/apt/lists/*` to keep the layer small.
- Add `set -o pipefail &&` before pipes so errors in piped commands are caught.

## COPY vs ADD

Prefer `COPY` for all local file transfers. `ADD` has implicit behavior (auto-extracts archives, fetches URLs) that makes Dockerfiles harder to reason about.

Use `ADD` only to fetch a remote artifact with checksum verification:

```dockerfile
ADD --checksum=sha256:abc123... \
    https://example.com/app.tar.gz /tmp/app.tar.gz
```

For everything else: `COPY`.

## CMD and ENTRYPOINT

Use exec form (JSON array) for both to avoid shell interpretation and ensure signals propagate to the process:

```dockerfile
ENTRYPOINT ["python", "-m", "gunicorn"]
CMD ["--bind", "0.0.0.0:8000", "app:app"]
```

`ENTRYPOINT` sets the fixed executable. `CMD` provides default arguments that callers can override. Use `ENTRYPOINT` alone (no CMD) when the image is purpose-built for one command.

Shell form (`CMD python app.py`) wraps the process in `/bin/sh -c`, which intercepts signals (SIGTERM). This causes graceful shutdown to fail in many runtimes. Always use exec form.

## USER and non-root execution

Create a non-root user and switch to it before `CMD`:

```dockerfile
RUN groupadd --gid 1001 appgroup \
  && useradd --uid 1001 --gid appgroup --shell /bin/bash --create-home appuser
USER appuser
```

Explicit UID/GID assignment makes the ownership deterministic when mounting volumes or when host systems need to match the container UID.

## WORKDIR

Always use absolute paths:

```dockerfile
WORKDIR /app
```

Do not use `RUN cd /some/dir` to change directories across instructions. `WORKDIR` is idempotent and creates the directory if it does not exist.

## .dockerignore

Place `.dockerignore` alongside the Dockerfile (or the build context root). It reduces build context size and prevents secrets from reaching the build:

```
.git
.env
.env.*
__pycache__
*.pyc
*.pyo
node_modules
.venv
venv
tests/
docs/
*.md
```

A large build context slows `docker build` because the entire context is sent to the daemon before the first instruction runs.

## References

- https://docs.docker.com/build/building/best-practices/
- https://docs.docker.com/reference/dockerfile/
