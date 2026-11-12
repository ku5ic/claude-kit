# Networking

- Legacy `--link`: replace it with a user-defined network. Its environment variables are off by default since Engine 29.0, and removal is slated for 30.0.

## Drivers

| Driver    | Isolation                                | Use when                                             |
| --------- | ---------------------------------------- | ---------------------------------------------------- |
| `bridge`  | Separate namespace, NAT to host          | Default; single-host services                        |
| `host`    | No isolation; uses host network directly | Performance-critical; not recommended for production |
| `none`    | Complete isolation; no network           | Batch jobs with no network needs                     |
| `overlay` | Multi-host; requires Swarm               | Multi-host Docker Swarm clusters                     |

## Service discovery

- Compose creates a user-defined bridge network for each project. Services on the same network reach each other by service name, as in `db:5432`.
- The default bridge network has no service name resolution; a user-defined network does.
- Aliases listed under `networks: <net>: aliases:` give a service a second name, and other services on that network reach it by either.

## Ports

- A published port is reachable from outside the Docker host and from containers on other bridge networks.
- `"8000:8000"` publishes on all interfaces, `"127.0.0.1:8000:8000"` on loopback only (the safer choice in development), and `"8080:80"` maps host 8080 to container 80.
- An internal service (database, cache) in production: no `ports:`. Services on its network reach it without one.

## Isolation

- Split services across networks so each joins only the ones it needs. With `frontend` on `public`, `api` on `public` and `backend`, and `db` on `backend`, `frontend` reaches `api` but not `db`, and `api` reaches both.
