# Firewall

## ufw basics

ufw (Uncomplicated Firewall) defaults to: incoming deny, forwarding deny, outgoing allow. This is the correct default for most servers.

```bash
ufw default deny incoming
ufw default allow outgoing
ufw default deny forward
```

## Allow necessary services

```bash
ufw allow ssh          # port 22
ufw allow http         # port 80
ufw allow https        # port 443
```

Or by port number:

```bash
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
```

Enable the firewall after adding rules:

```bash
ufw enable
```

Check status and rules:

```bash
ufw status verbose
ufw status numbered     # numbered for easier deletion
```

## Rate-limit SSH

`ufw limit` blocks a source IP after 6 connection attempts within 30 seconds:

```bash
ufw limit ssh
```

This defends against brute-force attacks without completely blocking SSH. Prefer this over a plain `allow ssh`.

## Restrict SSH to known IPs (when possible)

If your public IP is static, restrict SSH to that address:

```bash
ufw allow from 203.0.113.0/24 to any port 22
ufw deny 22
```

Order matters: `allow from` must come before `deny 22`.

## Delete rules

```bash
ufw status numbered
ufw delete 3           # delete rule number 3
ufw delete allow ssh   # delete by specification
```

## Reset to defaults

```bash
ufw reset              # disables ufw and removes all rules
```

## Application profiles

ufw includes application profiles for common software:

```bash
ufw app list           # show available profiles
ufw allow 'Nginx Full' # allows HTTP + HTTPS
ufw allow 'Nginx HTTP'
ufw allow 'Nginx HTTPS'
```

Use `Nginx Full` only during initial setup or before SSL is configured. In production, use `Nginx HTTPS` and redirect HTTP->HTTPS at the application level.

## References

- https://manpages.ubuntu.com/manpages/noble/man8/ufw.8.html
