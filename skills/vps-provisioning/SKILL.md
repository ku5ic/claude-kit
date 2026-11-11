---
name: vps-provisioning
description: VPS patterns for setup, firewall, nginx reverse proxy, Let's Encrypt, systemd, and hardening. Use for Ansible playbooks, provisioning scripts, nginx or systemd configs, or server questions.
---

# VPS provisioning patterns

Default assumption: Debian 13 (Trixie), Ubuntu 26.04 or 24.04 LTS.

- Debian 12 (Bookworm) is on LTS only, until 2028-06-30.
- Commands and package names are Debian/Ubuntu unless noted. Check them against the distribution's current docs.

## Reference files

| File                                           | Covers                                                                        |
| ---------------------------------------------- | ----------------------------------------------------------------------------- |
| [initial-setup.md](reference/initial-setup.md) | Non-root sudo user, SSH key auth, disabling root/password login               |
| [firewall.md](reference/firewall.md)           | ufw default policy, allow/deny rules, rate limiting                           |
| [nginx.md](reference/nginx.md)                 | Server blocks, HTTPS redirect, reverse proxy, gzip, security headers          |
| [ssl-tls.md](reference/ssl-tls.md)             | certbot 4.0.0 renewal threshold, issuance, auto-renewal, renewal hooks        |
| [systemd.md](reference/systemd.md)             | Unit file structure, Type values, Restart, EnvironmentFile, journald          |
| [hardening.md](reference/hardening.md)         | fail2ban 1.1.1 `sshd` backend, jails, unattended-upgrades, sshd config checks |
| [anti-patterns.md](reference/anti-patterns.md) | Thirteen review-time anti-patterns with severity calls                        |
