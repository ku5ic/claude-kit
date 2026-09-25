# Networking

## Network drivers

| Driver    | Isolation                                | Use when                                             |
| --------- | ---------------------------------------- | ---------------------------------------------------- |
| `bridge`  | Separate namespace, NAT to host          | Default; single-host services                        |
| `host`    | No isolation; uses host network directly | Performance-critical; not recommended for production |
| `none`    | Complete isolation; no network           | Batch jobs with no network needs                     |
| `overlay` | Multi-host; requires Swarm               | Multi-host Docker Swarm clusters                     |

## User-defined bridge networks

Compose creates a user-defined bridge network for each project automatically. Services on the same network communicate by service name:

```yaml
services:
  api:
    image: myapi
    networks:
      - backend

  db:
    image: postgres:16
    networks:
      - backend

networks:
  backend:
```

`api` can reach `db` at `db:5432`. The default bridge network does not support service name resolution; user-defined networks do.

## Port publishing

Publishing a port makes it reachable from outside the Docker host and from containers on other bridge networks.

```yaml
ports:
  - "8000:8000" # all interfaces, host 8000 -> container 8000
  - "127.0.0.1:8000:8000" # loopback only (more secure for dev)
  - "8080:80" # host 8080 -> container 80
```

Internal services (databases, caches) should NOT publish ports in production. A database with a published port is reachable from any process on the host.

```yaml
db:
  image: postgres:16
  # No `ports:` -- only reachable from services on the same network
  networks:
    - backend
```

## Multiple networks for isolation

Use separate networks to enforce that only certain services can communicate:

```yaml
services:
  frontend:
    networks: [public]

  api:
    networks: [public, backend]

  db:
    networks: [backend] # frontend cannot reach db directly

networks:
  public:
  backend:
```

`frontend` can reach `api`, but cannot reach `db`. `api` can reach both.

## DNS and service discovery

On a user-defined network, each container gets a DNS entry matching its service name. Aliases can be added:

```yaml
services:
  api:
    networks:
      backend:
        aliases:
          - api-service
```

Other services on `backend` can reach this container at `api` or `api-service`.

## References

- https://docs.docker.com/engine/network/
