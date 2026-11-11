# backup-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- https://restic.readthedocs.io/en/latest/
- https://www.postgresql.org/docs/current/app-pgdump.html
- https://www.postgresql.org/docs/current/app-pgrestore.html

### Version notes

Checked: 2026-11-10 against https://restic.readthedocs.io/en/stable/040_backup.html, https://github.com/restic/restic/releases, https://www.postgresql.org/support/versioning/

- restic 0.19.1 (2026-07-05): docs warn `--stdin` "cannot detect if data read from stdin is complete"; streaming example switched to `--stdin-from-command`. PostgreSQL 18.6 current, 14 oldest supported.
