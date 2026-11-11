# systemd services

## Unit file

- A unit file goes in `/etc/systemd/system/`, named `<service-name>.service`.
- `[Unit]`: `Description=My Application`, `After=network.target`.
- `[Service]` for an app server: `Type=exec`, `User=deploy`, `Group=deploy`, `WorkingDirectory=/opt/myapp`, `EnvironmentFile=/opt/myapp/.env`, `ExecStart=/opt/myapp/venv/bin/gunicorn --bind 127.0.0.1:8000 --workers 4 app:app`, `ExecReload=/bin/kill -s HUP $MAINPID`, `Restart=on-failure`, `RestartSec=5s`, `StandardOutput=journal`, `StandardError=journal`, `LimitNOFILE=65536`.
- `[Install]`: `WantedBy=multi-user.target`.

## Type values

| Type      | When to use                                                           |
| --------- | --------------------------------------------------------------------- |
| `exec`    | Most web servers and daemons (process stays in foreground)            |
| `simple`  | Same as exec but starts immediately (default; slightly less safe)     |
| `forking` | Traditional daemons that fork and exit parent process (nginx, apache) |
| `notify`  | Services that signal readiness via sd_notify (systemd-aware apps)     |
| `oneshot` | Scripts that run once and exit (e.g., init scripts, migrations)       |

- A process that doesn't fork: `Type=exec`. It waits for the exec syscall to succeed before marking the service started.

## Restart policies

| Value         | Restarts when                          |
| ------------- | -------------------------------------- |
| `on-failure`  | Non-zero exit or signal kill           |
| `always`      | Any exit (including clean exit code 0) |
| `on-abnormal` | Signal, timeout, or watchdog failure   |
| `no`          | Never (default)                        |

- Most services: `Restart=on-failure`. `always` restarts even after a clean exit.
- No policy restarts a service stopped with `systemctl stop`.

## EnvironmentFile

- `EnvironmentFile=/opt/myapp/.env` loads one `KEY=VALUE` per line. Lock the file down with `chown deploy:deploy /opt/myapp/.env` and `chmod 600 /opt/myapp/.env`.

## Managing the service

- After editing a unit file, run `systemctl daemon-reload`.
- Start on boot with `systemctl enable myapp`. `systemctl start`, `stop`, and `restart` take the same unit name.
- `systemctl reload myapp` sends SIGHUP when `ExecReload` is set. `systemctl status myapp` shows the current state and recent logs.
- Logs: `journalctl -u myapp`, with `-f` to follow, `--since "1 hour ago"`, `-n 100` for the last 100 lines, or `-p err` for errors only.

## Ordering and dependencies

- `After=network.target` orders the unit so it stops before the network goes down. It doesn't wait for the network at start. Never pair it with `Wants=` or `Requires=network.target`.
- A service that can't start until the network is configured: `After=network-online.target` and `Wants=network-online.target`, with the network manager's wait-online service enabled.
- `Requires=` is a hard dependency. If the required unit fails, this unit fails.
- Most web apps need only `After=network.target`. A service that can't start without its database gets `Requires=postgresql.service`.
