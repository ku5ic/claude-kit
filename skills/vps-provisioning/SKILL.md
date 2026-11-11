---
name: vps-provisioning
description: VPS patterns for setup, firewall, nginx reverse proxy, Let's Encrypt, systemd, and hardening. Use for Ansible playbooks, provisioning scripts, nginx or systemd configs, or server questions.
---

# VPS provisioning patterns

Default assumption: Debian 13 (Trixie), Ubuntu 26.04 or 24.04 LTS.

- Debian 12 (Bookworm) is on LTS only, until 2028-06-30.
- Commands and package names are Debian/Ubuntu unless noted. Check them against the distribution's current docs.

## Reference files

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have.

| Before                                                                                             | Read                                           |
| -------------------------------------------------------------------------------------------------- | ---------------------------------------------- |
| Setting up a fresh server: the sudo user, SSH keys, or disabling root and password login           | [initial-setup.md](reference/initial-setup.md) |
| Writing or changing ufw rules, the default policy, or rate limiting                                | [firewall.md](reference/firewall.md)           |
| Writing or changing an nginx server block, an HTTPS redirect, a reverse proxy, or security headers | [nginx.md](reference/nginx.md)                 |
| Issuing or renewing a certificate with certbot, or writing a renewal hook                          | [ssl-tls.md](reference/ssl-tls.md)             |
| Writing or changing a systemd unit, its restart policy, or its environment file                    | [systemd.md](reference/systemd.md)             |
| Configuring fail2ban, unattended-upgrades, or sshd hardening                                       | [hardening.md](reference/hardening.md)         |
| Reviewing provisioning code or server config                                                       | [anti-patterns.md](reference/anti-patterns.md) |
