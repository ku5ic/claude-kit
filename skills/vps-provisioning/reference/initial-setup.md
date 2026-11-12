# Initial setup

## Sudo user

- First login as root: create a regular user and grant sudo, as in `adduser deploy` and `usermod -aG sudo deploy`.
- Copy your SSH public key to the new user before disabling root login, with `ssh-copy-id deploy@<server-ip>` from your local machine.
- Copying the key by hand on the server: `/home/deploy/.ssh` at `chmod 700`, the key appended to `/home/deploy/.ssh/authorized_keys` at `chmod 600`, and `chown -R deploy:deploy /home/deploy/.ssh`.
- Log in as `deploy` with the SSH key before going further.

## SSH configuration

- `/etc/ssh/sshd_config`: `PasswordAuthentication no`, `PermitRootLogin no`, `PubkeyAuthentication yes`, `AuthorizedKeysFile .ssh/authorized_keys`, `X11Forwarding no`, `MaxAuthTries 3`, `LoginGraceTime 30`.
- After editing it, run `systemctl restart sshd`. Keep the current session open until a new session opens with the new settings.

## Keys

- A new key on the local machine: Ed25519, as in `ssh-keygen -t ed25519 -C "deploy@hostname" -f ~/.ssh/id_ed25519_deploy`. RSA 4096 is acceptable; RSA 1024/2048 and DSA are weak.

## Sudo for automation

- A script that runs specific privileged commands, optionally: a file in `/etc/sudoers.d/` naming only those commands, as in `deploy ALL=(ALL) NOPASSWD: /bin/systemctl restart myapp, /usr/bin/certbot renew` in `/etc/sudoers.d/deploy-limited`.
- Never grant blanket `NOPASSWD: ALL`.
- Editing a sudoers file: `visudo`, or `EDITOR=vim visudo -f /etc/sudoers.d/deploy-limited`.

## First updates

- Right after provisioning, run `apt-get update`, `apt-get upgrade -y`, `apt-get autoremove -y`, then `reboot`.
