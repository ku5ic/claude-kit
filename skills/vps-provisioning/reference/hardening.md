# Hardening

## Contents

- [fail2ban](#fail2ban)
- [Unattended security upgrades](#unattended-security-upgrades)
- [SSH hardening summary](#ssh-hardening-summary)
- [References](#references)

## fail2ban

Install and enable:

```bash
apt-get install fail2ban
systemctl enable fail2ban
systemctl start fail2ban
```

Create a local override at `/etc/fail2ban/jail.local` (never edit `jail.conf` directly; it is overwritten on upgrades):

```ini
[DEFAULT]
bantime  = 1h
findtime = 10m
maxretry = 5

[sshd]
enabled = true
port    = ssh
```

Check banned IPs and jail status:

```bash
fail2ban-client status
fail2ban-client status sshd
```

Unban an IP manually:

```bash
fail2ban-client set sshd unbanip 203.0.113.5
```

fail2ban monitors log files for repeated failures and adds iptables/nftables rules to block offending IPs. The `sshd` jail watches `/var/log/auth.log` by default on Debian/Ubuntu.

## Unattended security upgrades

Install and configure:

```bash
apt-get install unattended-upgrades
dpkg-reconfigure --priority=low unattended-upgrades
```

`dpkg-reconfigure` enables the service and writes `/etc/apt/apt.conf.d/20auto-upgrades`:

```
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
```

The main config is `/etc/apt/apt.conf.d/50unattended-upgrades`. The defaults apply security updates only. Edit `/etc/apt/apt.conf.d/52unattended-upgrades-local` to override without touching the package-managed file:

```
// Enable email on error
Unattended-Upgrade::Mail "you@example.com";
Unattended-Upgrade::MailOnlyOnError "true";

// Remove unused kernel packages after upgrade
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";

// Reboot automatically if required (e.g. kernel update)
Unattended-Upgrade::Automatic-Reboot "false";
```

Two systemd timers drive the process:

| Timer                     | Function                        |
| ------------------------- | ------------------------------- |
| `apt-daily.timer`         | Downloads updated package lists |
| `apt-daily-upgrade.timer` | Installs eligible upgrades      |

Verify both are active:

```bash
systemctl status apt-daily.timer
systemctl status apt-daily-upgrade.timer
```

Logs are in `/var/log/unattended-upgrades/`.

Dry run to preview what would be upgraded:

```bash
unattended-upgrade --dry-run --debug
```

## SSH hardening summary

Key settings in `/etc/ssh/sshd_config` (covered fully in initial-setup.md):

```
PasswordAuthentication no
PermitRootLogin no
PubkeyAuthentication yes
MaxAuthTries 3
LoginGraceTime 30
X11Forwarding no
```

Combine with `ufw limit ssh` (covered in firewall.md) and fail2ban sshd jail for layered defense.

Verify the running sshd config without restarting:

```bash
sshd -T | grep -E "passwordauthentication|permitrootlogin|pubkeyauthentication"
```

## References

- https://wiki.debian.org/UnattendedUpgrades
- https://help.ubuntu.com/community/AutomaticSecurityUpdates
- https://www.fail2ban.org/wiki/index.php/MANUAL_0_8
