# Nginx

## Reverse proxy

- The port 80 server block for a site holds only `return 301 https://$host$request_uri;`.
- The port 443 block: `listen 443 ssl;`, `ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;`, and `ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;`.
- In the 443 block: `server_tokens off;` to hide the nginx version, `client_max_body_size 20m;` for the upload size limit.
- Gzip: `gzip on;`, `gzip_vary on;`, and `gzip_types text/plain text/css application/json application/javascript text/xml;`.
- The proxied `location /`: `proxy_pass http://127.0.0.1:8000;`, `proxy_http_version 1.1;`, and `60s` for `proxy_read_timeout`, `proxy_connect_timeout`, and `proxy_send_timeout`.
- Its `proxy_set_header` lines: `Host $host`, `X-Real-IP $remote_addr`, `X-Forwarded-For $proxy_add_x_forwarded_for`, `X-Forwarded-Proto $scheme`.
- Enabling a site in `/etc/nginx/sites-available/myapp`: `ln -s /etc/nginx/sites-available/myapp /etc/nginx/sites-enabled/`, then `nginx -t` and `systemctl reload nginx`.

## Security headers

- In the `server {}` block, each `add_header` with `always`: `X-Frame-Options "SAMEORIGIN"`, `X-Content-Type-Options "nosniff"`, `Referrer-Policy "no-referrer-when-downgrade"`, `Strict-Transport-Security "max-age=31536000; includeSubDomains"`.
- `Strict-Transport-Security`: only after HTTPS is confirmed working. Once sent, browsers enforce it for the `max-age` duration.

## Static files

- Static files beside the proxy: `location /static/` with `alias /var/www/myapp/static/;`, `expires 1y;`, and `add_header Cache-Control "public, immutable";`, while `location /` keeps the `proxy_pass`.

## Rate limiting

- Per-IP request limits: `limit_req_zone $binary_remote_addr zone=api:10m rate=30r/m;` in the `http {}` block (`nginx.conf` or `conf.d/limits.conf`), and `limit_req zone=api burst=10 nodelay;` in a `server {}` or `location {}`, as in `location /api/` with `proxy_pass http://127.0.0.1:8000;`.

## Upstreams

- Several backends: `upstream app_servers` with `server 127.0.0.1:8000;`, `server 127.0.0.1:8001;`, and `keepalive 32;`. Its location sets `proxy_pass http://app_servers;`, `proxy_http_version 1.1;`, and `proxy_set_header Connection "";`.
