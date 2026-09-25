# Anti-patterns

## failure: root SSH login enabled

`PermitRootLogin yes` in sshd_config lets an attacker brute-force the one account that can do anything on the system. Set `PermitRootLogin no` and use a non-root sudo user.

## failure: password authentication enabled for SSH

`PasswordAuthentication yes` exposes the server to brute-force attacks over SSH. Set `PasswordAuthentication no` and use SSH keys exclusively.

## failure: firewall not enabled before exposing services

Starting a service without first configuring ufw rules leaves ports open to the internet. Enable ufw with deny-incoming default before starting any service, then allow only what is needed.

## failure: HTTP serving plaintext credentials or sensitive data

Running a web service on port 80 without redirecting to HTTPS sends session tokens, passwords, and API keys in cleartext. Always redirect HTTP -> HTTPS (`return 301 https://$host$request_uri`) and obtain a certificate before going live.

## failure: web app running as root

Running nginx, gunicorn, or any app server as root means a remote code execution vulnerability grants full system access. Use a dedicated non-root user (e.g. `deploy`) and set `User=deploy` in the systemd unit.

## warning: EnvironmentFile readable by other users

An `.env` file with `chmod 644` exposes database passwords and API keys to any local user. Set `chmod 600` and `chown deploy:deploy` on every EnvironmentFile.

## warning: ufw allow ssh without rate limiting

Plain `ufw allow ssh` permits unlimited connection attempts from any IP. Use `ufw limit ssh` instead; it blocks a source IP after 6 attempts within 30 seconds with no other configuration needed.

## warning: certbot timer not verified after install

On some minimal Debian installs the certbot systemd timer is not active, leaving certificates to expire at 90 days. Verify with `systemctl status certbot.timer` and add a cron fallback if the timer is absent.

## warning: nginx default site left enabled

The default nginx site at `/etc/nginx/sites-enabled/default` catches all unmatched requests. It can leak the nginx version and default page. Remove the symlink (`rm /etc/nginx/sites-enabled/default`) after creating your own server block.

## warning: systemd Restart=always on a service that exits intentionally

`Restart=always` restarts the service even after `systemctl stop`, causing it to immediately come back. Use `Restart=on-failure` for services that should stay stopped when explicitly halted.

## info: fail2ban not installed

Without fail2ban, repeated failed SSH logins generate no automatic ban. ufw rate limiting reduces exposure but fail2ban provides finer-grained per-jail controls and covers auth failures beyond SSH (e.g. nginx 401s if configured).

## info: unattended-upgrades not configured

A server receiving no automatic security updates accumulates known CVEs over time. Install and enable `unattended-upgrades` immediately after provisioning. Review `/var/log/unattended-upgrades/` weekly to confirm it is running.

## info: server_tokens on in nginx

Nginx sends its version number in the `Server` response header by default, making it easier to target known CVEs for that version. Add `server_tokens off` to the `http {}` or `server {}` block.
