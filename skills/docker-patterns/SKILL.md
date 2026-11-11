---
name: docker-patterns
description: Docker patterns for Dockerfiles, multi-stage builds, Compose, networking, and volumes. Use for a Dockerfile, a Compose file, .dockerignore, or container questions.
---

# Docker patterns

Default assumption: Docker Engine 29.9.3 and the Compose Specification, which supersedes the 2.x and 3.x formats.

## Reference files

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have.

| Before                                                                         | Read                                           |
| ------------------------------------------------------------------------------ | ---------------------------------------------- |
| Writing or changing a Dockerfile, or a `.dockerignore`                         | [dockerfile.md](reference/dockerfile.md)       |
| Splitting a build into stages, or using `COPY --from` or `--target`            | [multi-stage.md](reference/multi-stage.md)     |
| Writing or changing a Compose file, its services, env, or profiles             | [compose.md](reference/compose.md)             |
| Connecting containers, publishing ports, or choosing a network driver          | [networking.md](reference/networking.md)       |
| Persisting data with a volume, a bind mount, or tmpfs                          | [volumes.md](reference/volumes.md)             |
| Setting the user, pinning images, or handling secrets in an image or container | [security.md](reference/security.md)           |
| Reviewing Docker code                                                          | [anti-patterns.md](reference/anti-patterns.md) |
