---
name: backup-patterns
description: Backup patterns for the 3-2-1 rule, rsync, pg_dump, restic, retention, and restore tests. Use for backup scripts, cron or systemd backup jobs, or questions on backups or disaster recovery.
---

# Backup Patterns

Review checklist and the choices that are expensive to get wrong. Flag tables for `pg_dump`, `rsync`, and `restic` are in their man pages.

## 3-2-1

Keep three copies on two media, one of them offsite. A copy on the same server as the data doesn't count.

## Choices that matter

- Streaming a dump into restic: `restic -r <repo> backup --stdin-filename mydb.dump --stdin-from-command -- pg_dump -F c mydb`. It writes no plaintext temp file, and restic cancels the backup if `pg_dump` exits non-zero. A plain `pg_dump | restic backup --stdin` pipe can't detect a truncated dump.
- restic flags: check them against the installed version; `restic self-update` keeps the binary current.
- A database over a few hundred MB: dump it with `pg_dump -F c` (custom) or `-F d` (directory). Only these support `-j` parallel restore; `-F p` plain SQL is the slowest both ways.
- Roles and tablespaces, not just one database's contents: use `pg_dumpall`.
- `rsync --delete` deletes destination files missing from the source. Dry-run it with `-n` before the first real run.
- restic retention: `forget --keep-daily/--keep-weekly/--keep-monthly --prune`, with `--dry-run` first.

## Restore testing

On a schedule:

1. Restore to a separate location, such as `/tmp/restore` or a staging server.
2. Verify integrity: row counts, application startup, spot checks.
3. Record the restore duration, so the RTO is known before an incident.

## Anti-patterns

- A backup stored only on the same server as the data: `failure`. Keep an offsite copy, per 3-2-1.
- A backup never tested with a restore: `failure`. Restore-test it on a schedule.
- Plain SQL format (`-F p`) for a large database: `warning`. Use `-F c` or `-F d`.
- A pg_dump password in a shell script, as in `PGPASSWORD=secret pg_dump ...`: `warning`. Use `~/.pgpass` or a `PGSERVICE` entry in `pg_service.conf`.
- A restic password in the crontab or alongside the repo: `warning`. Keep it in a password or secret manager; in cron, source it from a protected env file, never inline.
- No retention policy on backups: `info`. Define and automate `forget`/`prune` retention from day one.

### Legacy (restic without --stdin-from-command)

Only on a restic build without `--stdin-from-command`, pipe `pg_dump -F c mydb | restic backup --stdin --stdin-filename mydb.dump` under `set -o pipefail`, then check the backup's size or run `pg_restore --list`.
