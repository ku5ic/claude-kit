# Docker Compose

## Contents

- [Service definition](#service-definition)
- [depends_on with conditions](#depends_on-with-conditions)
- [healthcheck](#healthcheck)
- [restart policies](#restart-policies)
- [Environment variables](#environment-variables)
- [Profiles](#profiles)
- [References](#references)

Current format: Compose Specification. The `version:` top-level key is ignored and should be omitted.

## Service definition

```yaml
services:
  api:
    build:
      context: .
      dockerfile: Dockerfile
      target: runtime
      args:
        BUILD_ENV: production
    image: myorg/api:1.0
    ports:
      - "127.0.0.1:8000:8000"
    env_file:
      - .env
    environment:
      DATABASE_URL: postgresql://db:5432/app
    depends_on:
      db:
        condition: service_healthy
    restart: unless-stopped
    networks:
      - backend
    volumes:
      - static:/app/static

  db:
    image: postgres:16
    environment:
      POSTGRES_DB: app
      POSTGRES_USER: appuser
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
    secrets:
      - db_password
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U appuser -d app"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 30s
    volumes:
      - pgdata:/var/lib/postgresql/data
    networks:
      - backend

volumes:
  pgdata:
  static:

networks:
  backend:

secrets:
  db_password:
    file: ./secrets/db_password.txt
```

## depends_on with conditions

`depends_on` alone only waits for the container to start (not to be ready). Use `condition: service_healthy` to wait for a passing healthcheck:

```yaml
depends_on:
  db:
    condition: service_healthy # waits for healthcheck to pass
  cache:
    condition: service_started # waits for container start only
```

`condition: service_healthy` requires the dependency to define a `healthcheck`. Without one, the condition is never satisfied and the dependent service does not start.

## healthcheck

```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
  interval: 30s # time between checks
  timeout: 10s # how long a check can take
  retries: 3 # consecutive failures to mark unhealthy
  start_period: 40s # grace period before failures count
```

Use `CMD-SHELL` form to run a shell command: `["CMD-SHELL", "pg_isready ..."]`. Use `CMD` form for direct exec without shell.

## restart policies

| Policy           | Behavior                                                                                   |
| ---------------- | ------------------------------------------------------------------------------------------ |
| `no`             | Never restart (default)                                                                    |
| `always`         | Always restart; also starts on daemon restart                                              |
| `unless-stopped` | Always restart unless manually stopped; does not start on daemon restart if it was stopped |
| `on-failure`     | Restart only on non-zero exit; optionally `on-failure:5` for max retries                   |

Prefer `unless-stopped` over `always` for long-running services: `always` restarts even containers you intentionally stopped, which is surprising in development.

## Environment variables

Three options, in increasing order of security:

1. `environment:` inline values (visible in `docker inspect`; use for non-sensitive config)
2. `env_file: [.env]` (file stays off CI/CD logs; still visible in inspect)
3. `secrets:` (mounted as files at `/run/secrets/<name>`; not in inspect output)

Load sensitive values from secrets, not environment variables.

## Profiles

Profiles restrict which services start by default:

```yaml
services:
  app:
    image: myapp
  debug-tools:
    image: busybox
    profiles: [debug]
```

`docker compose up` starts only `app`. `docker compose --profile debug up` also starts `debug-tools`. Use profiles for development-only services (mailhog, adminer, jaeger).

## References

- https://docs.docker.com/compose/compose-file/
- https://docs.docker.com/compose/compose-file/05-services/
