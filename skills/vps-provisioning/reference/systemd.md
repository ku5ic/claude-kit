# systemd Services

## Contents

- [Unit file structure](#unit-file-structure)
- [Type values](#type-values)
- [Restart policies](#restart-policies)
- [EnvironmentFile](#environmentfile)
- [Managing the service](#managing-the-service)
- [Viewing logs (journald)](#viewing-logs-journald)
- [After and Wants vs Requires](#after-and-wants-vs-requires)
- [References](#references)

## Unit file structure

Place unit files in `/etc/systemd/system/`. Name the file `<service-name>.service`.

```ini
[Unit]
Description=My Application
After=network.target
Wants=network.target

[Service]
Type=exec
User=deploy
Group=deploy
WorkingDirectory=/opt/myapp
EnvironmentFile=/opt/myapp/.env
ExecStart=/opt/myapp/venv/bin/gunicorn --bind 127.0.0.1:8000 --workers 4 app:app
ExecReload=/bin/kill -s HUP $MAINPID
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

## Type values

| Type      | When to use                                                           |
| --------- | --------------------------------------------------------------------- |
| `exec`    | Most web servers and daemons (process stays in foreground)            |
| `simple`  | Same as exec but starts immediately (default; slightly less safe)     |
| `forking` | Traditional daemons that fork and exit parent process (nginx, apache) |
| `notify`  | Services that signal readiness via sd_notify (systemd-aware apps)     |
| `oneshot` | Scripts that run once and exit (e.g., init scripts, migrations)       |

Use `exec` as the safe default for processes that do not fork. It waits for the exec syscall to succeed before marking the service started.

## Restart policies

| Value         | Restarts when                          |
| ------------- | -------------------------------------- |
| `on-failure`  | Non-zero exit or signal kill           |
| `always`      | Any exit (including clean exit code 0) |
| `on-abnormal` | Signal, timeout, or watchdog failure   |
| `no`          | Never (default)                        |

Use `on-failure` for most services. `always` restarts even on intentional `systemctl stop`.

## EnvironmentFile

Load environment variables from a file (one `KEY=VALUE` per line):

```ini
EnvironmentFile=/opt/myapp/.env
```

Set strict permissions on the env file:

```bash
chown deploy:deploy /opt/myapp/.env
chmod 600 /opt/myapp/.env
```

## Managing the service

```bash
systemctl daemon-reload          # after editing the unit file
systemctl enable myapp           # start on boot
systemctl start myapp
systemctl stop myapp
systemctl restart myapp
systemctl reload myapp           # send SIGHUP (if ExecReload is set)
systemctl status myapp           # current state and recent logs
```

## Viewing logs (journald)

```bash
journalctl -u myapp              # all logs for the unit
journalctl -u myapp -f           # follow (tail -f equivalent)
journalctl -u myapp --since "1 hour ago"
journalctl -u myapp -n 100       # last 100 lines
journalctl -u myapp -p err       # errors only
```

## After and Wants vs Requires

- `After=network.target`: start after networking is up (ordering only)
- `Wants=network.target`: weakly depends on another unit (starts without it if absent)
- `Requires=`: hard dependency; if the required unit fails, this unit fails

For most web apps: `After=network.target` is sufficient. `Requires=postgresql.service` is appropriate when the service cannot start without the database.

## References

- https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html
