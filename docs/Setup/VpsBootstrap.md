# VPS Bootstrap

How a fresh pilot VPS gets the layout the cron, deploy and restore scripts expect (S5-Q7, TechDebt O1). Nothing is edited on the VPS: `/opt/cassy` holds git checkouts pinned to a release tag, and secrets live only in `/etc/cassy`.

| Path | Content | Owner, mode |
|---|---|---|
| `/opt/cassy/CassyBackend` | `cassy-backend` at the release tag (scripts only; the API runs from the GHCR image) | root, read-only to `cassy` |
| `/opt/cassy/Docker` | `cassy-docker` at the same tag (`Core/DockerComposeProd.yml`, Nginx, cron) | root, read-only to `cassy` |
| `/opt/cassy/Docker/Core/.env` | symlink to `/etc/cassy/prod.env`, read by compose | |
| `/etc/cassy/prod.env`, `/etc/cassy/backup.env` | secrets from the password manager, never committed | `cassy`, `0600` |
| `/var/lib/cassy/deploy` | `current.env`, `previous.env`, `history.log` written by `DeployRelease.sh` | `cassy` |
| `/var/log/cassy` | backup, drill, purge and deploy logs | `cassy` |

**Release tags:** tag `cassy-backend` and `cassy-docker` with the same tag (for example `v2.6.0`) before bootstrapping or moving a VPS to a new release. The script checks out that tag in both.

## Steps

1. Base OS (OperationsRunbook.md section 1, steps 1-3): Ubuntu 24.04, user `cassy` with SSH key login only, Docker Engine, `ufw` with no inbound ports except SSH.
2. As root: `bash BootstrapVps.sh <release-tag>` (copy the script from the tag, or `curl` it from the tagged raw URL). It creates the layout above, installs `/etc/cron.d/cassy-backup`, and prints the two commit hashes. Re-run it with a newer tag to move both checkouts.
3. Create `/etc/cassy/prod.env` and `/etc/cassy/backup.env` (`install -m 600 -o cassy /dev/null <file>`, then fill from the password manager; field lists in `Deploy.md` and `BackupAndRestore.md` section 2). The script warns while either is missing.
4. As `cassy`: restore the newest backup if this replaces a lost VPS (`BackupAndRestore.md` section 3), otherwise skip.
5. As `cassy`: `scripts/deploy/DeployRelease.sh <api digest> [web digest]` from `/opt/cassy/CassyBackend`.
6. Cloudflare tunnel: point the tunnel token in `prod.env` at this VPS (only one connector may be active).
7. Smoke: `/health/ready` through the tunnel, one guest menu load, one POS login, then run the backup and the restore drill once by hand and keep the `RESULT PASS` line.

## RTO budget (target 2 hours, S5-Q7)

| Step | Budget | Measured |
|---|---|---|
| Provision VPS (Hostinger KVM 2) | 20 min | not yet (needs the pilot VPS) |
| Base OS, Docker, firewall | 15 min | not yet |
| `BootstrapVps.sh` | 2 min | not yet |
| Secrets from the password manager | 10 min | not yet |
| Restore newest backup (R2 download, GPG, `pg_restore`) | 10 min | 2026-11-25, local, demo DB 16 MB, 77 migrations: dump 3 s, fresh Postgres 18 ready 8 s, `pg_restore` 8 s, data intact |
| Deploy (image pull, migrate, health) | 10 min | API image pull 13 s locally |
| Tunnel switch and smoke | 15 min | not yet |
| **Total** | **82 min** | |

The database part is seconds at pilot sizes; the budget is dominated by provisioning and manual steps. Record the first full rehearsal on the pilot VPS here, with the real times, before go-live.
