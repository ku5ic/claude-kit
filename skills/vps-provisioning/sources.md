# vps-provisioning sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- https://certbot.eff.org/
- https://nginx.org/en/docs/
- https://wiki.debian.org/UnattendedUpgrades
- https://manpages.ubuntu.com/manpages/noble/man8/ufw.8.html

### Version notes

Checked: 2026-11-10 against https://ubuntu.com/about/release-cycle, https://www.debian.org/releases/, https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html, https://github.com/fail2ban/fail2ban/blob/master/ChangeLog and https://letsencrypt.org/2025/12/02/from-90-to-45

- 2026-11-11, four corrections. Production firewall said `Nginx HTTPS` only, which closes the port 80 redirect and HTTP-01 renewal the other files rely on. systemd units paired `After=` with `Wants=network.target`; https://systemd.io/NETWORK_ONLINE/ says `network.target` is for shutdown ordering and a unit that needs the network uses `network-online.target`. `X-XSS-Protection` is gone from the nginx headers; MDN marks it deprecated and says to use CSP. `Protocol 2` is gone from sshd_config; https://man.openbsd.org/sshd_config has no such keyword.

Check distribution LTS support windows before provisioning.

- Debian 13 and Ubuntu 26.04 LTS (2026-11-10): the new baseline. Corrected `Restart=always` (a `systemctl stop` is never restarted) and the fail2ban `sshd` backend on Debian.
- Ubuntu 24.04 LTS support ends May 2029 (standard), May 2034 (ESM), May 2039 (Legacy add-on). Corrected from April.
- Certbot 4.0.0: renewal threshold moved from a fixed 30 days to 1/3 of certificate lifetime; see ssl-tls.md.
- Certbot version and plugin names change; verify `certbot --version` and available plugins.

### Left to linters

Checked: 2026-11-11. No default check in the kit's set (go vet and golangci-lint defaults, ruff defaults, ESLint `js.configs.recommended`, shellcheck with no flags, `tsc --strict`) reads `sshd_config`, ufw rules, nginx or systemd configs, sudoers, or certbot and fail2ban setup, so no rule was dropped. The shell commands here are examples, and no rule is a shell-script rule shellcheck reports, so nothing was probed. `nginx -t`, `systemd-analyze verify`, `visudo`, ansible-lint, and certbot's own checks aren't in that set and don't count.

## reference/firewall.md

### References

- https://manpages.ubuntu.com/manpages/noble/man8/ufw.8.html

## reference/hardening.md

### References

- https://wiki.debian.org/UnattendedUpgrades
- https://help.ubuntu.com/community/AutomaticSecurityUpdates
- https://www.fail2ban.org/wiki/index.php/MANUAL_0_8

## reference/nginx.md

### References

- https://nginx.org/en/docs/http/ngx_http_proxy_module.html
- https://nginx.org/en/docs/http/ngx_http_core_module.html

## reference/ssl-tls.md

### References

- https://eff-certbot.readthedocs.io/using.html
- https://letsencrypt.org/docs/

## reference/systemd.md

### References

- https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html
