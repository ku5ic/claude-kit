# Anti-Patterns

## failure: secrets baked into the image

```dockerfile
ENV SECRET_KEY=abc123supersecret     # baked into image layer history
ARG API_KEY=mykey
RUN curl -H "X-API-Key: $API_KEY" https://api.example.com/setup  # in layer history
```

Secrets in `ENV` and `ARG` are visible in `docker history` and `docker inspect`. Any user with pull access to the image can read them. Inject secrets at runtime via Docker secrets or a secrets manager.

## failure: running as root

```dockerfile
FROM python:3.12-slim
COPY . .
CMD ["python", "app.py"]   # runs as root (uid 0)
```

Running as root means a container escape or RCE gives the attacker root on the host (if the container has excessive capabilities). Always add a `USER` instruction.

## failure: depends_on without healthcheck on condition: service_healthy

```yaml
services:
  api:
    depends_on:
      db:
        condition: service_healthy # will never be satisfied
  db:
    image: postgres:16 # no healthcheck defined
```

Without a `healthcheck` block on `db`, the container never enters the healthy state and `api` never starts. Either add a `healthcheck` to `db` or change the condition to `service_started`.

## failure: database port published to 0.0.0.0

```yaml
db:
  image: postgres:16
  ports:
    - "5432:5432" # reachable from anywhere on the host and network
```

A database port published to all interfaces is reachable from outside the Docker host if the firewall allows it. Do not publish internal service ports. Services on the same Compose network communicate directly without published ports.

## warning: apt-get update and install in separate RUN statements

```dockerfile
RUN apt-get update
RUN apt-get install -y curl   # may use stale cache from a previous build
```

If Docker has cached the `update` layer, the `install` may pull stale package lists and fail silently or install an outdated version. Always combine them in one `RUN`.

## warning: copying everything before installing dependencies

```dockerfile
COPY . .                  # WRONG order
RUN pip install -r requirements.txt
```

Any code change invalidates the pip cache. Put `COPY requirements.txt .` and `RUN pip install` before `COPY . .` so the install layer is cached as long as requirements do not change.

## warning: using `latest` tag in production

```dockerfile
FROM python:latest        # unpredictable; breaks on next major release
```

`latest` can silently change the base image on the next `docker pull`. Pin to a specific version tag.

## warning: no .dockerignore file

Without `.dockerignore`, the full build context (`.git`, `node_modules`, `.env`, test fixtures) is sent to the daemon on every build. This slows builds and may include secrets or large files that are not needed in the image.

## info: no multi-stage build for compiled or bundled apps

Single-stage builds for compiled languages (Go, Rust, TypeScript) include the compiler and all build dependencies in the final image. Use multi-stage to produce a minimal runtime image.

## info: shell form CMD

```dockerfile
CMD python app.py    # shell form: runs as /bin/sh -c "python app.py"
```

Shell form wraps the process in a shell. The shell becomes PID 1 and intercepts signals, preventing graceful shutdown. Use exec form: `CMD ["python", "app.py"]`.
