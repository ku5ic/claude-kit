# Nginx

## Contents

- [Reverse proxy server block](#reverse-proxy-server-block)
- [Security headers](#security-headers)
- [Serving static files with proxy fallback](#serving-static-files-with-proxy-fallback)
- [Rate limiting](#rate-limiting)
- [Upstream for multiple backends](#upstream-for-multiple-backends)
- [References](#references)

## Reverse proxy server block

```nginx
# /etc/nginx/sites-available/myapp
server {
    listen 80;
    server_name example.com www.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name example.com www.example.com;

    ssl_certificate     /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

    # Hide nginx version
    server_tokens off;

    # Upload size limit
    client_max_body_size 20m;

    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_types text/plain text/css application/json application/javascript text/xml;

    location / {
        proxy_pass         http://127.0.0.1:8000;
        proxy_http_version 1.1;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        proxy_read_timeout 60s;
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
    }
}
```

Enable the site:

```bash
ln -s /etc/nginx/sites-available/myapp /etc/nginx/sites-enabled/
nginx -t             # test config
systemctl reload nginx
```

## Security headers

Add to the `server {}` block:

```nginx
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "1; mode=block" always;
add_header Referrer-Policy "no-referrer-when-downgrade" always;
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
```

Set `Strict-Transport-Security` only after confirming HTTPS works correctly. Once sent, browsers enforce it for the `max-age` duration.

## Serving static files with proxy fallback

```nginx
location /static/ {
    alias /var/www/myapp/static/;
    expires 1y;
    add_header Cache-Control "public, immutable";
}

location / {
    proxy_pass http://127.0.0.1:8000;
    # ... proxy headers
}
```

## Rate limiting

Limit requests per IP to defend against abuse:

```nginx
# In http {} block (nginx.conf or conf.d/limits.conf)
limit_req_zone $binary_remote_addr zone=api:10m rate=30r/m;

# In server {} or location {}
location /api/ {
    limit_req zone=api burst=10 nodelay;
    proxy_pass http://127.0.0.1:8000;
}
```

## Upstream for multiple backends

```nginx
upstream app_servers {
    server 127.0.0.1:8000;
    server 127.0.0.1:8001;
    keepalive 32;
}

location / {
    proxy_pass http://app_servers;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    # ... other headers
}
```

## References

- https://nginx.org/en/docs/http/ngx_http_proxy_module.html
- https://nginx.org/en/docs/http/ngx_http_core_module.html
