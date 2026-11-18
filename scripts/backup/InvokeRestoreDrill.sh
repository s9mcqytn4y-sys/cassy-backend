#!/usr/bin/env bash
# ==============================================================================
# Cassy POS Platform - Monthly Restore Drill
# Restores the newest (or BACKUP_OBJECT) R2 backup into a throwaway PostgreSQL 18
# container, proves the API role can read it, compares migrations and row counts
# with the live database, prints one RESULT line, then removes the container.
# Never touches the live database except for read-only counts as cassy_backup.
#
# Usage (on the VPS, same env as BackupToR2.sh):
#   ./scripts/backup/InvokeRestoreDrill.sh >> /var/log/cassy/restore-drill.log 2>&1
# Optional: BACKUP_OBJECT (default latest), SOURCE_CONTAINER (default CassyPostgresProd),
#           SOURCE_DB (default DB_NAME or cassy_prod), PG_IMAGE (default postgres:18-alpine)
# ==============================================================================
set -euo pipefail
umask 077

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PG_IMAGE="${PG_IMAGE:-postgres:18-alpine}"
readonly SOURCE_CONTAINER="${SOURCE_CONTAINER:-CassyPostgresProd}"
readonly SOURCE_DB="${SOURCE_DB:-${DB_NAME:-cassy_prod}}"
readonly DRILL_DB="cassy_drill"
readonly DRILL_CONTAINER="cassy-restore-drill-$$"
readonly STARTED_AT="$(date -u +%s)"

TEMP_INIT_SCRIPT=""
if [[ -n "${INIT_SCRIPT:-}" && -f "${INIT_SCRIPT}" ]]; then
  RESOLVED_INIT_SCRIPT="${INIT_SCRIPT}"
elif [[ -f "${SCRIPT_DIR}/../../../Docker/Postgres/InitCassyRoles.sh" ]]; then
  RESOLVED_INIT_SCRIPT="${SCRIPT_DIR}/../../../Docker/Postgres/InitCassyRoles.sh"
else
  TEMP_INIT_SCRIPT="$(mktemp)"
  chmod 0755 "${TEMP_INIT_SCRIPT}"
  cat > "${TEMP_INIT_SCRIPT}" <<'EOS'
#!/bin/sh
set -eu
psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname "${POSTGRES_DB}" <<-SQL
  DO \$\$
  BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${DB_MIGRATOR_USER:-cassy_migrator}') THEN
      CREATE ROLE ${DB_MIGRATOR_USER:-cassy_migrator} WITH LOGIN PASSWORD '${DB_MIGRATOR_PASSWORD:-cassy_migrator}' NOSUPERUSER NOCREATEDB NOCREATEROLE;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${DB_APP_USER:-cassy_app}') THEN
      CREATE ROLE ${DB_APP_USER:-cassy_app} WITH LOGIN PASSWORD '${DB_APP_PASSWORD:-cassy_app}' NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${DB_BACKUP_USER:-cassy_backup}') THEN
      CREATE ROLE ${DB_BACKUP_USER:-cassy_backup} WITH LOGIN PASSWORD NULL NOSUPERUSER NOCREATEDB NOCREATEROLE BYPASSRLS;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'mycaffe_app') THEN
      CREATE ROLE mycaffe_app WITH NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
    END IF;
  END
  \$\$;
  GRANT ALL PRIVILEGES ON DATABASE ${POSTGRES_DB} TO ${DB_MIGRATOR_USER:-cassy_migrator};
  ALTER SCHEMA public OWNER TO ${DB_MIGRATOR_USER:-cassy_migrator};
  GRANT ALL ON SCHEMA public TO ${DB_MIGRATOR_USER:-cassy_migrator};
  GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO ${DB_APP_USER:-cassy_app};
  GRANT USAGE ON SCHEMA public TO ${DB_APP_USER:-cassy_app};
  GRANT ${DB_APP_USER:-cassy_app} TO ${DB_MIGRATOR_USER:-cassy_migrator} WITH INHERIT FALSE, SET TRUE;
  GRANT mycaffe_app TO ${DB_MIGRATOR_USER:-cassy_migrator} WITH INHERIT FALSE, SET TRUE;
  GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO ${DB_BACKUP_USER:-cassy_backup};
  GRANT pg_read_all_data TO ${DB_BACKUP_USER:-cassy_backup};
  CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
SQL
EOS
  RESOLVED_INIT_SCRIPT="${TEMP_INIT_SCRIPT}"
fi

for tool in docker aws gpg sha256sum openssl; do
  command -v "${tool}" >/dev/null || { echo "[DRILL ERROR] ${tool} is required." >&2; exit 1; }
done

cleanup() {
  docker rm -f "${DRILL_CONTAINER}" >/dev/null 2>&1 || true
  if [[ -n "${TEMP_INIT_SCRIPT}" ]]; then
    rm -f "${TEMP_INIT_SCRIPT}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

echo "[DRILL] Starting throwaway ${PG_IMAGE} container ${DRILL_CONTAINER}..."
# docker create + cp instead of a bind mount: works the same on Linux hosts and Git Bash.
docker create --name "${DRILL_CONTAINER}" \
  -e POSTGRES_PASSWORD="$(openssl rand -hex 24)" \
  -e POSTGRES_DB="${DRILL_DB}" \
  -e DB_MIGRATOR_USER=cassy_migrator -e DB_MIGRATOR_PASSWORD="$(openssl rand -hex 24)" \
  -e DB_APP_USER=cassy_app -e DB_APP_PASSWORD="$(openssl rand -hex 24)" \
  "${PG_IMAGE}" >/dev/null
docker cp "${RESOLVED_INIT_SCRIPT}" "${DRILL_CONTAINER}:/docker-entrypoint-initdb.d/00-init-roles.sh"
docker start "${DRILL_CONTAINER}" >/dev/null

for _ in $(seq 1 60); do
  if docker logs "${DRILL_CONTAINER}" 2>&1 | grep -q 'PostgreSQL init process complete' \
     && docker exec "${DRILL_CONTAINER}" pg_isready -q -U cassy_migrator -d "${DRILL_DB}"; then
    break
  fi
  sleep 1
done
docker exec "${DRILL_CONTAINER}" pg_isready -q -U cassy_migrator -d "${DRILL_DB}" \
  || { echo "[DRILL ERROR] Throwaway PostgreSQL did not become ready." >&2; exit 1; }

BACKUP_OBJECT="${BACKUP_OBJECT:-latest}" DB_CONTAINER="${DRILL_CONTAINER}" DB_NAME="${DRILL_DB}" \
  DB_MIGRATOR_USER=cassy_migrator APP_ENV=drill \
  "${SCRIPT_DIR}/RestoreFromR2.sh"

q() { # container, user, db, sql
  docker exec "$1" psql -X -q -t -A -v ON_ERROR_STOP=1 -U "$2" -d "$3" -c "$4"
}

echo "[DRILL] Checking that cassy_app can read the restored schema (GRANTs restored)..."
q "${DRILL_CONTAINER}" cassy_app "${DRILL_DB}" "SELECT count(*) FROM orders" >/dev/null

COUNT_SQL="SELECT (SELECT count(*) FROM schema_migrations)||' '||(SELECT count(*) FROM stores)||' '||(SELECT count(*) FROM orders)||' '||(SELECT count(*) FROM payments)||' '||(SELECT count(*) FROM audit_logs)"
read -r R_MIG R_STORES R_ORDERS R_PAYMENTS R_AUDIT <<< "$(q "${DRILL_CONTAINER}" postgres "${DRILL_DB}" "${COUNT_SQL}")"

STATUS="PASS"
if docker inspect "${SOURCE_CONTAINER}" >/dev/null 2>&1; then
  read -r L_MIG L_STORES L_ORDERS L_PAYMENTS L_AUDIT <<< "$(q "${SOURCE_CONTAINER}" cassy_backup "${SOURCE_DB}" "${COUNT_SQL}")"
  echo "[DRILL] live     : migrations=${L_MIG} stores=${L_STORES} orders=${L_ORDERS} payments=${L_PAYMENTS} audit_logs=${L_AUDIT}"
  # The backup may be up to an hour old: fewer rows is expected, more rows or a schema mismatch is not.
  if [[ "${R_MIG}" != "${L_MIG}" ]] || (( R_STORES > L_STORES || R_ORDERS > L_ORDERS || R_PAYMENTS > L_PAYMENTS || R_AUDIT > L_AUDIT )); then
    STATUS="FAIL"
  fi
else
  echo "[DRILL] Live container ${SOURCE_CONTAINER} not found; skipping live comparison."
fi
echo "[DRILL] restored : migrations=${R_MIG} stores=${R_STORES} orders=${R_ORDERS} payments=${R_PAYMENTS} audit_logs=${R_AUDIT}"
if (( R_MIG == 0 )); then
  STATUS="FAIL"
fi

DURATION=$(( $(date -u +%s) - STARTED_AT ))
echo "RESULT ${STATUS} at=$(date -u +%Y-%m-%dT%H:%M:%SZ) object=${BACKUP_OBJECT:-latest} duration_s=${DURATION} migrations=${R_MIG} orders=${R_ORDERS}"

# Only a passing drill moves the CassyRestoreDrillStale clock (Alloy textfile collector).
METRICS_DIR="${METRICS_TEXTFILE_DIR:-/var/lib/cassy/metrics}"
if [[ "${STATUS}" == "PASS" && -d "${METRICS_DIR}" ]]; then
  printf '# TYPE cassy_restore_drill_last_success_timestamp_seconds gauge\ncassy_restore_drill_last_success_timestamp_seconds %s\n' \
    "$(date -u +%s)" > "${METRICS_DIR}/restore_drill.prom.$$" && mv "${METRICS_DIR}/restore_drill.prom.$$" "${METRICS_DIR}/restore_drill.prom"
fi
[[ "${STATUS}" == "PASS" ]]
