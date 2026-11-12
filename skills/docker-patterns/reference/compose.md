# Docker Compose

- The Compose Specification ignores the top-level `version:` key. Omit it.

## Services and builds

- A service built from a multi-stage Dockerfile: set `build.target` to the runtime stage, and pass build arguments under `build.args`.
- A database password: mount it as a Compose secret and point the image at the file, as in `POSTGRES_PASSWORD_FILE: /run/secrets/db_password`.

## depends_on and healthcheck

- `depends_on` alone waits only for the dependency's container to start. To wait until it's ready, set `condition: service_healthy`; `condition: service_started` waits for the start only.
- `condition: service_healthy` needs a `healthcheck` on the dependency. Without one, the condition is never met and the dependent service never starts.
- `healthcheck` fields: `test`, `interval` between checks, `timeout` for one check, `retries` (consecutive failures before unhealthy), and `start_period` (grace period before failures count).
- A healthcheck test that needs a shell uses the `CMD-SHELL` form, as in `["CMD-SHELL", "pg_isready ..."]`, and one that execs directly uses the `CMD` form.

## Restart policies

| Policy           | Behavior                                                                                   |
| ---------------- | ------------------------------------------------------------------------------------------ |
| `no`             | Never restart (default)                                                                    |
| `always`         | Always restart; also starts on daemon restart                                              |
| `unless-stopped` | Always restart unless manually stopped; does not start on daemon restart if it was stopped |
| `on-failure`     | Restart only on non-zero exit; optionally `on-failure:5` for max retries                   |

- A long-running service: `unless-stopped`, not `always`.

## Environment and secrets

From least to most secure:

1. `environment:` inline values, visible in `docker inspect`. Only for non-sensitive config.
2. `env_file: [.env]`, which keeps values out of CI/CD logs but not out of `docker inspect`.
3. `secrets:`, mounted as files at `/run/secrets/<name>` and absent from `docker inspect`.

- Sensitive values: `secrets:`, never environment variables.

## Profiles

- A development-only service (mailhog, adminer, jaeger): give it `profiles: [debug]`. `docker compose up` skips it, and `docker compose --profile debug up` starts it too.
