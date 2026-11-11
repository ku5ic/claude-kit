---
name: docker-patterns
description: Docker patterns for Dockerfiles, multi-stage builds, Compose, networking, and volumes. Use for a Dockerfile, a Compose file, .dockerignore, or container questions.
---

# Docker patterns

Docker Engine 29.9.3. Compose Specification (current format, supersedes 2.x and 3.x).

## Reference files

| File                                           | Covers                                                                                       |
| ---------------------------------------------- | -------------------------------------------------------------------------------------------- |
| [dockerfile.md](reference/dockerfile.md)       | Base image selection, layer caching, RUN/COPY/ADD/CMD/ENTRYPOINT/USER/WORKDIR, .dockerignore |
| [multi-stage.md](reference/multi-stage.md)     | Multi-stage builds, named stages, COPY --from, --target, BuildKit                            |
| [compose.md](reference/compose.md)             | Services, build, ports, depends_on with healthcheck, restart, env_file, profiles, secrets    |
| [networking.md](reference/networking.md)       | Bridge/host/none drivers, user-defined networks, service discovery, port publishing          |
| [volumes.md](reference/volumes.md)             | Named volumes vs bind mounts vs tmpfs, when to use each, Compose volume declarations         |
| [security.md](reference/security.md)           | Non-root user, minimal base images, image pinning, .dockerignore, Docker Scout, secrets      |
| [anti-patterns.md](reference/anti-patterns.md) | Severity-labeled anti-patterns to flag in review                                             |
