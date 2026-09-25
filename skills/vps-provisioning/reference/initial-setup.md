# Initial Setup

## Create a non-root sudo user

Log in as root initially, then create a regular user and grant sudo:

```bash
adduser deploy
usermod -aG sudo deploy
```

Copy your SSH public key to the new user before disabling root login:

```bash
# From your local machine
ssh-copy-id deploy@<server-ip>

# Or manually on the server
mkdir -p /home/deploy/.ssh
chmod 700 /home/deploy/.ssh
echo "ssh-ed25519 AAAA... your-key" >> /home/deploy/.ssh/authorized_keys
chmod 600 /home/deploy/.ssh/authorized_keys
chown -R deploy:deploy /home/deploy/.ssh
```

Verify you can log in as `deploy` with the SSH key before proceeding.

## Harden SSH configuration

Edit `/etc/ssh/sshd_config`:

```
PasswordAuthentication no
PermitRootLogin no
PubkeyAuthentication yes
AuthorizedKeysFile .ssh/authorized_keys
Protocol 2
X11Forwarding no
MaxAuthTries 3
LoginGraceTime 30
```

Restart SSH after editing:

```bash
systemctl restart sshd
```

Do not close your current session until you have verified you can open a new session with the new settings.

## Key generation (local machine)

Use Ed25519 keys. RSA 4096 is acceptable. RSA 1024/2048 and DSA are weak.

```bash
ssh-keygen -t ed25519 -C "deploy@hostname" -f ~/.ssh/id_ed25519_deploy
```

## Sudo without password for specific commands (optional)

For automation scripts that need to run specific privileged commands:

```
# /etc/sudoers.d/deploy-limited
deploy ALL=(ALL) NOPASSWD: /bin/systemctl restart myapp, /usr/bin/certbot renew
```

Use `visudo` or `EDITOR=vim visudo -f /etc/sudoers.d/deploy-limited` to edit sudoers files safely. Do not grant blanket `NOPASSWD: ALL`.

## System updates after first login

```bash
apt-get update
apt-get upgrade -y
apt-get autoremove -y
reboot
```

Run this immediately after provisioning to ensure the system starts with current packages.
