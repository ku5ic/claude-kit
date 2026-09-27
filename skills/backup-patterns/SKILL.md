---
name: backup-patterns
description: >
  Backup patterns for Linux servers and applications covering the 3-2-1 rule,
  rsync file backups, PostgreSQL dumps with pg_dump, encrypted backups with
  restic, retention policies, and restore testing. Use whenever the project
  contains shell scripts using pg_dump, rsync, or restic, a Brewfile or
  requirements file with restic or pgbackup tooling, cron job definitions for
  backups, or backup-related systemd units, OR the user asks about backups,
  disaster recovery, data retention, pg_dump, rsync, or restic, even if "backup"
  is not mentioned by name.
---

# Backup Patterns

Review checklist and the choices that are expensive to get wrong. Flag tables for `pg_dump`, `rsync`, and `restic` are in their man pages.

## 3-2-1

Three copies, two media, one offsite. A backup that lives only on the same server as the data is not a backup.

## Choices that matter

- **`pg_dump -F c`** (custom) or **`-F d`** (directory) for anything over a few hundred MB. Only these support `-j` parallel restore; `-F p` plain SQL is the slowest both ways.
- **`pg_dumpall`** when you need roles and tablespaces, not just one database's contents.
- **Stream rather than stage**: `restic -r <repo> backup --stdin-filename mydb.dump --stdin-from-command -- pg_dump -F c mydb` avoids a plaintext temp file, and restic cancels the backup if `pg_dump` exits non-zero. A plain `pg_dump | restic backup --stdin` pipe cannot detect a truncated dump.
- **`rsync --delete`** mirrors exactly - it deletes destination files missing from the source. Dry-run with `-n` before the first real run.
- **restic retention** is `forget --keep-daily/--keep-weekly/--keep-monthly --prune`. Without it, storage grows unboundedly. `--dry-run` first.

## Restore testing

A backup never tested is not a backup. On a schedule:

1. Restore to a separate location (`/tmp/restore`, a staging server).
2. Verify integrity - row counts, application startup, spot checks.
3. Record the restore duration, so the RTO is known before an incident rather than during one.

---

## Anti-patterns

**failure: backup stored only on the same server as the data**
A hardware failure, ransomware infection, or accidental `rm -rf` destroys both. The offsite copy in 3-2-1 exists for exactly this case.

**failure: backup never tested with a restore**
A corrupted or incomplete backup discovered during an incident is worse than no backup, because it delays the decision to use other recovery options.

**warning: plain SQL format for large databases**
`-F p` is the slowest to dump and restore and cannot use `-j` parallel workers. Use `-F c` or `-F d`.

**warning: pg_dump password in a shell script**
`PGPASSWORD=secret pg_dump ...` leaks the password into process listings. Use `~/.pgpass` or a `PGSERVICE` entry in `pg_service.conf`.

**warning: restic password in the crontab or alongside the repo**
restic encryption is strong enough that a lost password means permanent data loss, and a password stored next to the data defeats the encryption. Keep it in a password manager or secret manager, and source it from a protected env file in cron - never inline.

**info: no retention policy on backups**
Without `forget`/`prune`, backup storage grows unboundedly. Define and automate retention from day one.

---

## References

- https://restic.readthedocs.io/en/latest/
- https://www.postgresql.org/docs/current/app-pgdump.html
- https://www.postgresql.org/docs/current/app-pgrestore.html

> Verify restic flags against the version you have installed; restic self-update keeps the binary current.

## Version notes

Checked: 2026-09-26 against https://restic.readthedocs.io/en/stable/040_backup.html, https://github.com/restic/restic/releases, https://www.postgresql.org/support/versioning/

- restic 0.19.1 (2026-07-05): docs warn `--stdin` "cannot detect if data read from stdin is complete"; streaming example switched to `--stdin-from-command`. PostgreSQL 18.6 current, 14 oldest supported.

### Legacy (restic without --stdin-from-command)

The earlier example piped `pg_dump -F c mydb | restic backup --stdin --stdin-filename mydb.dump`. Only use it on a restic build that lacks `--stdin-from-command`, and add `set -o pipefail` plus a post-backup size or `pg_restore --list` check, since restic records whatever arrived on stdin.
