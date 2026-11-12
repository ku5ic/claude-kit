# SSL/TLS

- certbot 4.0.0+: `certbot renew` acts only once less than 1/3 of the certificate's lifetime remains, or 1/2 for a certificate valid 10 days or less. The twice-daily timer keeps working as Let's Encrypt shortens lifetimes.
- certbot below 4.0.0, as in older distro-packaged builds: the renewal threshold is a fixed 30 days before expiry. Check `certbot --version` before relying on the lifetime fraction.

## Issuance

- Install certbot and the nginx plugin: `apt-get install certbot python3-certbot-nginx`.
- Obtaining and installing in one step, with certbot editing the nginx config: `certbot --nginx -d example.com -d www.example.com`.
- Obtaining only, to configure nginx by hand: `certbot certonly --nginx -d example.com -d www.example.com`.
- Certificates land in `/etc/letsencrypt/live/example.com/`: `fullchain.pem` (certificate plus intermediate chain) for `ssl_certificate`, and `privkey.pem` (private key) for `ssl_certificate_key`.

## Authenticator plugins

| Plugin                       | How it works                                  | Requires                   |
| ---------------------------- | --------------------------------------------- | -------------------------- |
| `--nginx`                    | Temporarily modifies nginx to serve challenge | nginx running              |
| `--standalone`               | Spins up a temporary HTTP server              | Port 80 free               |
| `--webroot -w /var/www/html` | Places files in existing webroot              | Webserver serving that dir |

- Renewing without stopping nginx: `--webroot`. `--standalone` only when nginx isn't running.

## Renewal

- On Debian/Ubuntu, installing certbot adds a systemd timer (`certbot.timer`) or a cron job. Check it with `systemctl status certbot.timer`.
- No timer: add `0 */12 * * * root certbot renew --quiet` to `/etc/cron.d/certbot`.
- A dry run without renewing: `certbot renew --dry-run`.

## Renewal hooks

| Directory                                | Runs when                                   |
| ---------------------------------------- | ------------------------------------------- |
| `/etc/letsencrypt/renewal-hooks/pre/`    | Before any renewal attempt                  |
| `/etc/letsencrypt/renewal-hooks/deploy/` | After each successful renewal               |
| `/etc/letsencrypt/renewal-hooks/post/`   | After renewal attempts (success or failure) |

- Reloading nginx after renewal: a hook in `deploy/`, such as `/etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh`, a `#!/bin/bash` script running `systemctl reload nginx`, made executable with `chmod +x`.

## Certificate management

- `certbot certificates` lists managed certificates, `certbot delete --cert-name example.com` removes one, and `certbot revoke --cert-name example.com` revokes and removes it.

## nginx

- After issuance, point `ssl_certificate` and `ssl_certificate_key` at those paths, and restrict TLS with `ssl_protocols TLSv1.2 TLSv1.3;` and `ssl_prefer_server_ciphers off;`.
