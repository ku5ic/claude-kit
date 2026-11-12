# Firewall

- ufw defaults to incoming deny, forwarding deny, outgoing allow. Keep that on most servers: `ufw default deny incoming`, `ufw default allow outgoing`, `ufw default deny forward`.
- Allowing a service: by name, `ufw allow ssh` (port 22), `ufw allow http` (80), `ufw allow https` (443), or by port, `ufw allow 22/tcp`, `80/tcp`, `443/tcp`.
- After adding rules: `ufw enable`.
- Checking rules: `ufw status verbose`, or `ufw status numbered` for the numbers to delete by.
- SSH: `ufw limit ssh` over a plain `ufw allow ssh`. It blocks a source IP after 6 connection attempts within 30 seconds.
- A static public IP: restrict SSH to it with `ufw allow from 203.0.113.0/24 to any port 22`, then `ufw deny 22`. The `allow from` rule comes before `deny 22`.
- Deleting a rule: by number, as in `ufw delete 3`, or by specification, as in `ufw delete allow ssh`.
- `ufw reset` disables ufw and removes all rules.
- Application profiles: `ufw app list` shows them. `ufw allow 'Nginx Full'` allows HTTP and HTTPS; `'Nginx HTTP'` and `'Nginx HTTPS'` allow one each.
- `Nginx Full` while nginx redirects port 80 to HTTPS or certbot renews through `--nginx` or `--webroot`. `Nginx HTTPS` alone only when nothing needs port 80.
