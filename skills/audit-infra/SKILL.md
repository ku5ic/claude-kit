---
name: audit-infra
description: Review a target's infrastructure files against the kit's checklists
argument-hint: "<file, directory, or none for the whole repo>"
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
---

# /audit-infra

Review of the infrastructure files a target holds against the kit's own checklists.

## Run

Resolve a linked ticket or doc as `skills/investigate` step 0 describes, then work this file's procedure on the resolved arguments per `skills/kit-rules/reference/subagents.md` section 4, each part in a `kit:auditor` or one auditor for all of it, and write the report yourself. Then run the report-format skill's Runtime checks yourself, and reply per `skills/kit-rules/reference/output.md` section 3.

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
3. CI workflows are `/audit-ci`'s job. Name it as the next step when `.github/workflows` exists.
4. Anything that needs the live system (an open port, a running container's user, the state file) goes under "Needs a runtime check". Run no `plan`, `apply`, `docker build`, or provisioning tool here; a `plan` can be offered afterwards as a runtime check.

## Not a finding

- A default password in a service only a dev profile or a dev-named file starts. Name it as `info`.
- A rule the loaded reference marks optional or situational, when the file's own comment gives the reason.

## Severity

- `failure`: a committed secret, a database or admin port open to every address, or a container that runs as root and takes network input.
- `warning`: an unpinned base image or provider version, or a service with no health check or restart policy.
- `info`: hygiene with no exposure, such as a missing `.dockerignore` entry for docs.

## Output per finding

- What it is, with `file:line`
- Evidence: the quoted lines and the reference rule they break
- Severity: per the report-format skill's rubric
- Fix: the replacement lines

## Output file

Load the report-format skill and use its format. Write to the path `kit scratch-dir infra <target-slug>` prints.

Arguments: `$ARGUMENTS`
