---
name: vps-provisioning
description: VPS provisioning patterns for Linux servers covering initial setup, firewall, nginx reverse proxy, SSL/TLS with Let's Encrypt, systemd service management, and server hardening. Use whenever the project contains Ansible playbooks, shell provisioning scripts, nginx configs, systemd unit files, or certbot references, OR the user asks about VPS setup, server hardening, ufw, fail2ban, nginx reverse proxy, certbot, Let's Encrypt, systemd services, unattended-upgrades, even if VPS is not mentioned by name.
---

# VPS provisioning patterns

Targeting Debian 12 (Bookworm) and Ubuntu 24.04 LTS. Commands and package names are Debian/Ubuntu unless noted. Verify against your distribution's current docs.

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

## References

- https://certbot.eff.org/
- https://nginx.org/en/docs/
- https://wiki.debian.org/UnattendedUpgrades
- https://manpages.ubuntu.com/manpages/noble/man8/ufw.8.html

## Version notes

Check distribution LTS support windows before provisioning.

- Ubuntu 24.04 LTS support ends April 2029 (standard), April 2034 (ESM).
- Certbot version and plugin names change; verify `certbot --version` and available plugins.
