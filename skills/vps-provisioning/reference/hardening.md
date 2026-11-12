# Hardening

## fail2ban

- fail2ban 1.1.1+ on Debian paths runs the `sshd` jail on the `systemd` backend (the journal). Debian 12 dropped rsyslog from the default install, and `/var/log/auth.log` may not exist.
- Install with `apt-get install fail2ban`, then `systemctl enable fail2ban` and `systemctl start fail2ban`.
- Local settings go in `/etc/fail2ban/jail.local`. Never edit `jail.conf`, which upgrades overwrite.
- A `jail.local` to start from: `[DEFAULT]` with `bantime = 1h`, `findtime = 10m`, and `maxretry = 5`, and `[sshd]` with `enabled = true` and `port = ssh`.
- Jail status and banned IPs: `fail2ban-client status` and `fail2ban-client status sshd`. Unban by hand with `fail2ban-client set sshd unbanip 203.0.113.5`.
- fail2ban watches log files for repeated failures and blocks offending IPs with iptables/nftables rules.

## Unattended security upgrades

- Install with `apt-get install unattended-upgrades`, then `dpkg-reconfigure --priority=low unattended-upgrades`. It enables the service and writes `APT::Periodic::Update-Package-Lists "1";` and `APT::Periodic::Unattended-Upgrade "1";` to `/etc/apt/apt.conf.d/20auto-upgrades`.
- The main config, `/etc/apt/apt.conf.d/50unattended-upgrades`, applies security updates only by default. Override it in `/etc/apt/apt.conf.d/52unattended-upgrades-local`, never in the package-managed file.
- Mail on error: `Unattended-Upgrade::Mail "you@example.com";` and `Unattended-Upgrade::MailOnlyOnError "true";`.
- Removing unused kernel packages and dependencies after an upgrade: `Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";` and `Unattended-Upgrade::Remove-Unused-Dependencies "true";`.
- `Unattended-Upgrade::Automatic-Reboot "false";` sets whether to reboot automatically when required, as after a kernel update.
- `apt-daily.timer` downloads updated package lists and `apt-daily-upgrade.timer` installs eligible upgrades. Check both with `systemctl status apt-daily.timer` and `systemctl status apt-daily-upgrade.timer`.
- Logs are in `/var/log/unattended-upgrades/`. Preview a run with `unattended-upgrade --dry-run --debug`.

## SSH

- Combine the `/etc/ssh/sshd_config` settings in `initial-setup.md` with `ufw limit ssh` (`firewall.md`) and the fail2ban `sshd` jail.
- Checking the running sshd config without a restart: `sshd -T | grep -E "passwordauthentication|permitrootlogin|pubkeyauthentication"`.
