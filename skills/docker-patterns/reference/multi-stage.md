# Multi-Stage Builds

Multi-stage builds separate build-time dependencies from the runtime image. The final image contains only what the application needs to run, leaving behind compilers, test tools, and dev dependencies.

## Named stages

Name each stage with `AS`:

```dockerfile
FROM node:22-slim AS deps
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production

FROM node:22-slim AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:22-slim AS runtime
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY --from=build /app/dist ./dist
USER node
CMD ["node", "dist/server.js"]
```

`COPY --from=stagename` copies files from a named stage. Numeric indices (`--from=0`) work but break when stages are reordered.

## Python example

```dockerfile
FROM python:3.12-slim AS builder
WORKDIR /app
RUN pip install --no-cache-dir build
COPY . .
RUN python -m build --wheel

FROM python:3.12-slim AS runtime
WORKDIR /app
RUN groupadd --gid 1001 appgroup \
  && useradd --uid 1001 --gid appgroup appuser
COPY --from=builder /app/dist/*.whl /tmp/
RUN pip install --no-cache-dir /tmp/*.whl
USER appuser
CMD ["python", "-m", "myapp"]
```

## Targeting a specific stage

Build only up to a named stage for development or debugging:

```bash
docker build --target build -t myapp:build .
```

Useful when the `build` stage has test tooling and you want to run tests inside it:

```bash
docker build --target build -t myapp:test .
docker run --rm myapp:test pytest
```

## Copying from external images

`COPY --from` accepts any image reference, not just local stages:

```dockerfile
COPY --from=nginx:1.27 /etc/nginx/nginx.conf /etc/nginx/nginx.conf
```

## BuildKit parallel execution

With BuildKit (the default since Docker 23.0), independent stages that do not depend on each other can execute in parallel. Structure the Dockerfile so the `deps` stage (slow, network-bound) and a `test-data` preparation stage run concurrently.

## Stage naming convention

| Stage name | Purpose                                    |
| ---------- | ------------------------------------------ |
| `deps`     | Install only runtime dependencies          |
| `build`    | Compile, bundle, or generate artifacts     |
| `test`     | Run test suite (never used as final stage) |
| `runtime`  | Final production image                     |

The `test` stage exists purely to run tests inside CI (`--target test`). It is never used as the base for `runtime`.

## References

- https://docs.docker.com/build/building/multi-stage/
