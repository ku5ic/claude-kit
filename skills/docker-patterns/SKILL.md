---
name: docker-patterns
description: Docker patterns for Dockerfiles, multi-stage builds, Compose, networking, and volumes. Use for a Dockerfile, a Compose file, .dockerignore, or container questions.
---

# Docker patterns

Default assumption: Docker Engine 29.9.3 and the Compose Specification, which supersedes the 2.x and 3.x formats.

## Reference files

| File                                           | Covers                                                                                     |
| ---------------------------------------------- | ------------------------------------------------------------------------------------------ |
| [dockerfile.md](reference/dockerfile.md)       | Base images, layer order, `apt-get`, COPY vs ADD, exec form, USER, WORKDIR, .dockerignore  |
| [multi-stage.md](reference/multi-stage.md)     | BuildKit parallel stages, named stages, `COPY --from`, `--target`, stage names             |
| [compose.md](reference/compose.md)             | `version:` key, depends_on with healthcheck, restart policies, env and secrets, profiles   |
| [networking.md](reference/networking.md)       | Legacy `--link`, drivers, service discovery and aliases, port publishing, isolation        |
| [volumes.md](reference/volumes.md)             | Named volumes vs bind mounts vs tmpfs, Compose declarations, `down -v`                     |
| [security.md](reference/security.md)           | Non-root user, minimal and pinned images, build context, runtime secrets, Scout, read-only |
| [anti-patterns.md](reference/anti-patterns.md) | Ten review-time anti-patterns with severity calls                                          |
