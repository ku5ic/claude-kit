# Anti-patterns

Severity levels: the report-format skill.

- `PermitRootLogin yes` in `sshd_config`: `failure`. Set `PermitRootLogin no` and use a non-root sudo user.
- `PasswordAuthentication yes` for SSH: `failure`. Set `PasswordAuthentication no` and use SSH keys exclusively.
- A service started before ufw rules are configured: `failure`. Enable ufw with a deny-incoming default before starting any service, then allow only what's needed.
- A web service on port 80 serving credentials or sensitive data with no redirect to HTTPS: `failure`. Redirect HTTP to HTTPS (`return 301 https://$host$request_uri`) and obtain a certificate before going live.
- nginx, gunicorn, or any app server running as root: `failure`. Use a dedicated non-root user, such as `deploy`, and set `User=deploy` in the systemd unit.
- An `EnvironmentFile` readable by other users, as with `chmod 644`: `warning`. Set `chmod 600` and `chown deploy:deploy` on every EnvironmentFile.
- `ufw allow ssh` without rate limiting: `warning`. Use `ufw limit ssh`, which blocks a source IP after 6 attempts within 30 seconds.
- The certbot timer not verified after install: `warning`. On some minimal Debian installs it isn't active, and certificates expire (90 days on Let's Encrypt's default profile, 64 from 2027-02-10). Check `systemctl status certbot.timer`, and add a cron fallback if the timer is absent.
- The nginx default site left enabled: `warning`. Remove the symlink with `rm /etc/nginx/sites-enabled/default` after creating your own server block.
- `Restart=always` on a service that exits 0 to signal it's done: `warning`. Use `Restart=on-failure` for a service that should stay down after a clean exit.
- No fail2ban: `info`. Install it. Its jails give finer per-jail control than ufw rate limiting and can cover auth failures beyond SSH, such as nginx 401s.
- unattended-upgrades not configured: `info`. Install and enable `unattended-upgrades` right after provisioning, and review `/var/log/unattended-upgrades/` weekly.
- `server_tokens` on in nginx: `info`. Add `server_tokens off` to the `http {}` or `server {}` block.
