# Multi-stage builds

- With BuildKit, the default builder since Docker 23.0, stages that don't depend on each other run in parallel. Keep a slow, network-bound `deps` stage independent of other preparation stages, such as `test-data`, so they run concurrently.
- Compilers, test tools, and dev dependencies stay in build stages. The final stage holds only what the application needs at runtime.
- Name every stage with `AS`, and copy from it with `COPY --from=<name>`, never a numeric index such as `--from=0`.
- `COPY --from` also takes any image reference: `COPY --from=nginx:1.27 /etc/nginx/nginx.conf /etc/nginx/nginx.conf`.
- Building only up to one stage, for development, debugging, or tests: `docker build --target <stage>`. To run tests in a `build` stage that has the test tooling: `docker build --target build -t myapp:test .`, then `docker run --rm myapp:test pytest`.

## Stage names

| Stage name | Purpose                                    |
| ---------- | ------------------------------------------ |
| `deps`     | Install only runtime dependencies          |
| `build`    | Compile, bundle, or generate artifacts     |
| `test`     | Run test suite (never used as final stage) |
| `runtime`  | Final production image                     |

- The `test` stage runs only in CI, through `--target test`, and is never the base for `runtime`.

## Shapes

- Node: a `deps` stage runs `npm ci --omit=dev`, a `build` stage runs `npm ci` and `npm run build`, and the `runtime` stage copies `node_modules` from `deps` and `dist` from `build`, then switches to `USER node`.
- Python: a `builder` stage runs `python -m build --wheel`. The `runtime` stage creates a non-root user, copies the wheel with `COPY --from=builder`, installs it with `pip install --no-cache-dir`, and switches to that user.
