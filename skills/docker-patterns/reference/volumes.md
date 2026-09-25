# Volumes

## Contents

- [Three storage types](#three-storage-types)
- [Named volumes](#named-volumes)
- [Bind mounts](#bind-mounts)
- [tmpfs](#tmpfs)
- [Volume lifecycle](#volume-lifecycle)
- [References](#references)

## Three storage types

| Type         | Managed by      | Data persists                   | Use when                                         |
| ------------ | --------------- | ------------------------------- | ------------------------------------------------ |
| Named volume | Docker daemon   | Yes, survives container removal | Production data, shared data between containers  |
| Bind mount   | Host filesystem | Yes (it's a host path)          | Development: live code reload, local data access |
| tmpfs        | Host RAM        | No, lost when container stops   | Temporary caches, sensitive in-memory data       |

## Named volumes

Named volumes are managed by Docker. They survive `docker compose down` (but not `docker compose down -v`).

Declare at the top level and reference from services:

```yaml
services:
  db:
    image: postgres:16
    volumes:
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7
    volumes:
      - redisdata:/data

volumes:
  pgdata: # minimal declaration; Docker chooses the driver
  redisdata:
```

To specify a driver or options:

```yaml
volumes:
  pgdata:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /mnt/fast-disk/pgdata
```

## Bind mounts

Bind mounts mirror a host path directly into the container. Used in development for live code reload:

```yaml
services:
  api:
    image: myapi:dev
    volumes:
      - type: bind
        source: ./src
        target: /app/src
        read_only: false
```

Short form: `- ./src:/app/src`.

Do not use bind mounts for database data directories in production. Container restart with a different UID can change file ownership and break the service.

## tmpfs

Mount a RAM-backed filesystem for data that must not be written to disk:

```yaml
services:
  worker:
    image: myworker
    tmpfs:
      - /tmp
      - /run
```

Or with options (size limit):

```yaml
volumes:
  - type: tmpfs
    target: /tmp
    tmpfs:
      size: 100m
```

## Volume lifecycle

```bash
docker volume ls                   # list all volumes
docker volume inspect pgdata       # inspect a named volume
docker volume rm pgdata            # remove (fails if in use)
docker compose down -v             # stop and remove all project volumes
```

`docker compose down` without `-v` preserves named volumes. This is intentional: down is a normal stop, not a data wipe.

## References

- https://docs.docker.com/engine/storage/
