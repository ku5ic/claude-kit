# SSL/TLS

## Contents

- [Obtaining a certificate with certbot](#obtaining-a-certificate-with-certbot)
- [Authenticator plugins](#authenticator-plugins)
- [Auto-renewal](#auto-renewal)
- [Renewal hooks](#renewal-hooks)
- [Testing renewal](#testing-renewal)
- [Certificate management](#certificate-management)
- [Nginx SSL configuration](#nginx-ssl-configuration)
- [References](#references)

## Obtaining a certificate with certbot

Install certbot and the nginx plugin:

```bash
apt-get install certbot python3-certbot-nginx
```

Obtain and install in one step (certbot modifies nginx config automatically):

```bash
certbot --nginx -d example.com -d www.example.com
```

Or obtain only, then configure nginx manually:

```bash
certbot certonly --nginx -d example.com -d www.example.com
```

Certificates are stored at `/etc/letsencrypt/live/example.com/`:

- `fullchain.pem` - certificate + intermediate chain (use for `ssl_certificate`)
- `privkey.pem` - private key (use for `ssl_certificate_key`)

## Authenticator plugins

| Plugin                       | How it works                                  | Requires                   |
| ---------------------------- | --------------------------------------------- | -------------------------- |
| `--nginx`                    | Temporarily modifies nginx to serve challenge | nginx running              |
| `--standalone`               | Spins up a temporary HTTP server              | Port 80 free               |
| `--webroot -w /var/www/html` | Places files in existing webroot              | Webserver serving that dir |

Use `--webroot` when you want to renew without stopping nginx. Use `--standalone` only if nginx is not running.

## Auto-renewal

On Debian/Ubuntu, certbot installs a systemd timer (`certbot.timer`) or cron job automatically on installation. Verify:

```bash
systemctl status certbot.timer
```

If not present, add to cron:

```bash
# /etc/cron.d/certbot
0 */12 * * * root certbot renew --quiet
```

certbot renew only acts if a certificate expires within 30 days.

## Renewal hooks

Scripts in these directories run automatically around renewals:

| Directory                                | Runs when                                   |
| ---------------------------------------- | ------------------------------------------- |
| `/etc/letsencrypt/renewal-hooks/pre/`    | Before any renewal attempt                  |
| `/etc/letsencrypt/renewal-hooks/deploy/` | After each successful renewal               |
| `/etc/letsencrypt/renewal-hooks/post/`   | After renewal attempts (success or failure) |

Reload nginx after renewal by placing a script in `deploy/`:

```bash
# /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh
#!/bin/bash
systemctl reload nginx
```

```bash
chmod +x /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh
```

## Testing renewal

Dry run without actually renewing:

```bash
certbot renew --dry-run
```

## Certificate management

```bash
certbot certificates              # list all managed certificates
certbot delete --cert-name example.com    # remove a certificate
certbot revoke --cert-name example.com    # revoke and remove
```

## Nginx SSL configuration

After obtaining the certificate, reference the paths in nginx:

```nginx
ssl_certificate     /etc/letsencrypt/live/example.com/fullchain.pem;
ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

# Recommended: restrict to modern TLS versions
ssl_protocols TLSv1.2 TLSv1.3;
ssl_prefer_server_ciphers off;
```

## References

- https://eff-certbot.readthedocs.io/using.html
- https://letsencrypt.org/docs/
