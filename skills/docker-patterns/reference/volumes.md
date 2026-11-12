# Volumes

| Type         | Managed by      | Data persists                   | Use when                                         |
| ------------ | --------------- | ------------------------------- | ------------------------------------------------ |
| Named volume | Docker daemon   | Yes, survives container removal | Production data, shared data between containers  |
| Bind mount   | Host filesystem | Yes (it's a host path)          | Development: live code reload, local data access |
| tmpfs        | Host RAM        | No, lost when container stops   | Temporary caches, sensitive in-memory data       |

## Named volumes

- Declare a named volume under the top-level `volumes:` key and reference it from a service, as in `pgdata:/var/lib/postgresql/data`. A bare `pgdata:` declaration leaves the driver to Docker.
- A named volume on a chosen host device: `driver: local` with `driver_opts` set to `type: none`, `o: bind`, and `device: <host path>`.
- `docker compose down` keeps named volumes; `docker compose down -v` removes every volume of the project.
- `docker volume rm` fails on a volume that's in use.

## Bind mounts

- Live code reload in development: a bind mount, short form `./src:/app/src`, or long form `type: bind` with `source`, `target`, and `read_only`.
- Never bind-mount a database data directory in production.

## tmpfs

- Data that must not touch disk: a tmpfs mount, either the service's `tmpfs:` list (`/tmp`, `/run`) or a `volumes:` entry with `type: tmpfs`, a `target`, and `tmpfs: size:` (such as `100m`) to cap it.
