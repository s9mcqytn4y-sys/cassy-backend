# Backup & Restore Runbook (per client VPS)

Target: **RPO ≤ 1 hour, RTO ≤ 2 hours** (audit ticket P0-02, tightened by S5-Q7; budget in [VpsBootstrap.md](VpsBootstrap.md)). Status: scripts and CI round-trip *Implemented & Tested*; first real drill on a client VPS *Planned* (no VPS yet).

## 1. What runs

| Job | Schedule (UTC) | Script | Log |
|-----|----------------|--------|-----|
| Encrypted backup | every hour at :15 | `scripts/backup/BackupToR2.sh` | `/var/log/cassy/backup.log` |
| Restore drill | 2nd of month 20:30 (03:30 WIB) | `scripts/backup/InvokeRestoreDrill.sh` | `/var/log/cassy/restore-drill.log` |

Both are installed from `Docker/Backup/CassyBackup.cron` (repo `cassy-docker`) and share one `flock`, so a drill never overlaps a backup.

**Backup**: `pg_dump -Fc` inside `CassyPostgresProd` as `cassy_backup` (read-only, `BYPASSRLS`, no password, local socket only) → GPG AES-256 → SHA-256 manifest → R2 `backups/hourly/`. The first backup of each UTC day and month is copied to `backups/daily/` and `backups/monthly/`.

**Drill**: restores the newest hourly backup into a throwaway `postgres:18-alpine` container, checks that `cassy_app` can read the tables (GRANTs restored), compares migrations and row counts with the live database, prints `RESULT PASS|FAIL ... duration_s=...`, and deletes the container. The live database is only read.

## 2. One-time setup on a new VPS

1. R2 bucket per client, e.g. `cassy-backups-<client>`, with an API token limited to that bucket (Object Read & Write).
2. R2 lifecycle rules on that bucket (Cloudflare dashboard → R2 → bucket → Settings → Object lifecycle rules):

   | Prefix | Delete after |
   |--------|--------------|
   | `backups/hourly/` | 2 days |
   | `backups/daily/` | 30 days |
   | `backups/monthly/` | 365 days |

3. Host packages: `docker`, `gpg`, `awscli` (v2), `openssl`, `util-linux` (`flock`).
4. Run `scripts/deploy/BootstrapVps.sh <release-tag>` as root ([VpsBootstrap.md](VpsBootstrap.md)): user `cassy` in the `docker` group, `/opt/cassy` = git checkouts of both repos at the tag, `/var/log/cassy` and `/var/lib/cassy`. Then `mkdir -p /var/lib/cassy/metrics && chown cassy /var/lib/cassy/metrics` (the metrics directory feeds the backup alerts, see [Monitoring.md](Monitoring.md)).
5. `/etc/cassy/backup.env` (owner `cassy`, mode `600`), values from the password manager, never committed:

   ```bash
   DB_CONTAINER=CassyPostgresProd
   DB_NAME=cassy_prod
   R2_ENDPOINT=https://<account_id>.r2.cloudflarestorage.com
   R2_BACKUP_BUCKET=cassy-backups-<client>
   R2_BACKUP_PREFIX=backups
   AWS_ACCESS_KEY_ID=<r2 token id>
   AWS_SECRET_ACCESS_KEY=<r2 token secret>
   AWS_DEFAULT_REGION=auto
   BACKUP_ENCRYPTION_PASSPHRASE=<high-entropy passphrase, also stored offline>
   ```

   Losing the passphrase makes every backup unreadable. Keep a copy outside the VPS.

6. `install -m 644 /opt/cassy/Docker/Backup/CassyBackup.cron /etc/cron.d/cassy-backup (done by `BootstrapVps.sh`)`
7. As user `cassy`, run the two commands from the cron file once by hand and keep the drill's `RESULT PASS` line.

The `cassy_backup` role is created by `../Docker/Postgres/InitCassyRoles.sh` on the first start of an empty data volume. On a volume initialised before this change, run that script once as the superuser (it is idempotent).

## 3. Disaster recovery (VPS lost)

1. New VPS: §2 steps 3–5, then `docker compose -f deploy/docker/docker-compose.prod.yml up -d postgres` with the same `.env` (roles are created on the empty volume).
2. Restore the newest backup into it:

   ```bash
   set -a; . /etc/cassy/backup.env; set +a
   BACKUP_OBJECT=latest DB_MIGRATOR_USER=cassy_migrator APP_ENV=production CONFIRM_DESTRUCTIVE_RESTORE=true \
     /opt/cassy/scripts/backup/RestoreFromR2.sh
   ```

   A specific backup: `BACKUP_OBJECT=daily/backup_cassy_prod_20261102_001500Z.dump.gpg`.
3. Start the API with the same image digest as before (`migrate -action=up` is a no-op if the backup is current), point the Cloudflare Tunnel (IN1) at the new VPS, run the smoke test.
4. Record the incident: time lost (RPO actually achieved), time to recover (RTO), backup object used.

## 4. Why the restore keeps privileges and skips extensions

- Privileges: restoring with `--no-privileges` drops the GRANTs to `cassy_app`; the API then fails with `permission denied for table orders` (reproduced 2026-11-02). The script restores them.
- Extensions: `uuid-ossp` belongs to the superuser and is created by `00-init-roles.sh`; `--clean` would try to drop it as `cassy_migrator` and abort the restore, so extension entries are filtered out of the restore list.

## 5. Not covered yet

- RPO below 1 hour (WAL archiving): ticket P2-08, only if a client contract requires it.
