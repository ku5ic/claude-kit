# docker-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- https://docs.docker.com/build/building/best-practices/
- https://docs.docker.com/compose/compose-file/
- https://docs.docker.com/engine/network/
- https://docs.docker.com/engine/storage/

### Version notes

Checked: 2026-11-10 against https://docs.docker.com/engine/release-notes/29/, https://docs.docker.com/engine/deprecated/, https://docs.docker.com/engine/containers/start-containers-automatically/ and https://docs.docker.com/compose/compose-file/04-version-and-name/

Docker Engine and Docker Compose release independently. The Compose Specification supersedes the versioned format (version: "3"), and the `version` top-level key is now ignored.

- 29.9.3 (2026-11-03): corrected the `always` restart policy per the docs, and noted legacy link environment variables are disabled since 29.0, removal targeted for 30.0.
- 29.8.1 (2026-09-15): bug fixes only (containerd image store, logging, networking); no guidance change.
- 29.4.2 (2026-05-01): previous baseline.

### Claims

- Docker patterns, Docker Engine 29.9.3. Compose Specification (current form...: https://docs.docker.com/engine/release-notes/, https://docs.docker.com/compose/compose-file/

## reference/compose.md

### References

- https://docs.docker.com/compose/compose-file/
- https://docs.docker.com/compose/compose-file/05-services/

## reference/dockerfile.md

### References

- https://docs.docker.com/build/building/best-practices/
- https://docs.docker.com/reference/dockerfile/

## reference/multi-stage.md

### References

- https://docs.docker.com/build/building/multi-stage/

## reference/networking.md

### References

- https://docs.docker.com/engine/network/

## reference/security.md

### References

- https://docs.docker.com/build/building/best-practices/
- https://docs.docker.com/scout/

## reference/volumes.md

### References

- https://docs.docker.com/engine/storage/
