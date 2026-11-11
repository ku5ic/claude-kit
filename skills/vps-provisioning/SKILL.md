---
name: vps-provisioning
description: VPS patterns for setup, firewall, nginx reverse proxy, Let's Encrypt, systemd, and hardening. Use for Ansible playbooks, provisioning scripts, nginx or systemd configs, or server questions.
---

# VPS provisioning patterns

Targeting Debian 13 (Trixie) and Ubuntu 24.04 or 26.04 LTS. Debian 12 (Bookworm) is on LTS only, until 2028-06-30. Commands and package names are Debian/Ubuntu unless noted. Verify against your distribution's current docs.

## Reference files

| File                                           | Covers                                                               |
| ---------------------------------------------- | -------------------------------------------------------------------- |
| [initial-setup.md](reference/initial-setup.md) | Non-root sudo user, SSH key auth, disabling root/password login      |
| [firewall.md](reference/firewall.md)           | ufw default policy, allow/deny rules, rate limiting                  |
| [nginx.md](reference/nginx.md)                 | Server blocks, HTTPS redirect, reverse proxy, gzip, security headers |
| [ssl-tls.md](reference/ssl-tls.md)             | certbot, Let's Encrypt issuance, auto-renewal, renewal hooks         |
| [systemd.md](reference/systemd.md)             | Unit file structure, Type values, Restart, EnvironmentFile, journald |
| [hardening.md](reference/hardening.md)         | fail2ban, unattended-upgrades, SSH config, server_tokens             |
| [anti-patterns.md](reference/anti-patterns.md) | Severity-labeled anti-patterns to flag in review                     |
