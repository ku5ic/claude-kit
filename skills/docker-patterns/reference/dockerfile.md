# Dockerfile

## Base image

- Pick a base image from Docker Official Images or Verified Publisher images, the smallest one that fits.
- Production Dockerfiles: no `latest` tag. Pin a specific version or a digest.

| Base                   | Size    | Use case                                         |
| ---------------------- | ------- | ------------------------------------------------ |
| `alpine:3`             | ~6 MB   | General purpose; musl libc (check compatibility) |
| `debian:bookworm-slim` | ~75 MB  | When glibc is required                           |
| `python:3.12-slim`     | ~130 MB | Python apps with fewer system packages           |
| `distroless/python3`   | ~50 MB  | Production: no shell, no package manager         |
| `scratch`              | 0       | Statically compiled binaries only (Go, Rust)     |

## Layers and RUN

- Order instructions from least to most often changed: system packages, then dependencies, then application code.
- Copy the dependency manifest (`requirements.txt`) and install from it before `COPY . .`.
- `apt-get update` and `apt-get install` go in the same `RUN`.
- `apt-get install`: pass `--no-install-recommends`, and end the same `RUN` with `rm -rf /var/lib/apt/lists/*`.
- Prefix a `RUN` that pipes commands with `set -o pipefail &&`.

## Instructions

- Local files: `COPY`. `ADD` only to fetch a remote artifact with `--checksum=sha256:...`.
- `CMD` and `ENTRYPOINT`: exec form (a JSON array), never shell form (`CMD python app.py`), which runs under `/bin/sh -c`.
- `ENTRYPOINT` sets the fixed executable and `CMD` the default arguments a caller can override. An image built for one command: `ENTRYPOINT` alone.
- Before `CMD`, create a non-root user with an explicit UID and GID (`groupadd --gid 1001`, `useradd --uid 1001`) and switch to it with `USER`.
- `WORKDIR`: an absolute path, never `RUN cd` to change directory across instructions. `WORKDIR` creates the directory if it's missing.

## .dockerignore

- Put `.dockerignore` beside the Dockerfile or at the build context root.
- List `.git`, `.env` and `.env.*`, `node_modules`, `.venv` and `venv`, `__pycache__`, `*.pyc` and `*.pyo`, `tests/`, `docs/`, and `*.md`.
