# /audit infra

Review of the infrastructure files a target holds against the kit's own checklists. Arguments: `<file, directory, or none for the whole repo>`.

## Prerequisites

- Find the files with `fd` in the target. If it holds none of the kinds below, stop and say so.

## Procedure

1. For each kind present, load its skill and read its `anti-patterns.md` first, then every reference its table names for what the files do:

| Files                                                                                            | Skill               |
| ------------------------------------------------------------------------------------------------ | ------------------- |
| `Dockerfile*`, `*.dockerfile`, `compose*.y*ml`, `docker-compose*.y*ml`, `.dockerignore`          | `docker-patterns`   |
| `*.tf`, `*.tfvars`, `.terraform.lock.hcl`                                                        | `opentofu-patterns` |
| nginx config, systemd units, firewall rules, Ansible playbooks, cloud-init, provisioning scripts | `vps-provisioning`  |
| backup scripts, and the cron or systemd timer jobs that run them                                 | `backup-patterns`   |

2. Read each file and check it against the loaded references. A secret committed in one of these files is a `failure`, named redacted.
3. CI workflows are `audit ci`'s job. Name it as the next step when `.github/workflows` exists.
4. Anything that needs the live system (an open port, a running container's user, the state file) goes under "Cannot be verified statically". Never run `plan`, `apply`, `docker build`, or a provisioning tool.

## Output per finding

- What it is, with `file:line`
- Evidence: the quoted lines and the reference rule they break
- Severity: per the report-format skill's rubric
- Fix: the replacement lines

## Output file

Load the report-format skill and use its format. Write to the path `kit scratch-dir infra <target-slug>` prints.
