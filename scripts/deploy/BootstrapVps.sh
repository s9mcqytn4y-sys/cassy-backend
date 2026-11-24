#!/usr/bin/env bash
# Usage (as root on a fresh Ubuntu VPS with Docker installed): BootstrapVps.sh <release-tag>
# Lays out a VPS the way the cron and deploy scripts expect (S5-Q7, TechDebt O1):
#   /opt/cassy/CassyBackend and /opt/cassy/Docker are git checkouts pinned to <release-tag>, read-only to
#   the deploy user; secrets live only in /etc/cassy/*.env (0600); runtime state in /var/lib/cassy and logs
#   in /var/log/cassy. Re-running with a newer tag moves both checkouts to it. Never edits code on the VPS.
set -euo pipefail

TAG="${1:?usage: BootstrapVps.sh <release-tag, e.g. v1.4.0>}"
DEPLOY_USER="${DEPLOY_USER:-cassy}"
ROOT="/opt/cassy"
ORG="https://github.com/s9mcqytn4y-sys"

[[ "$(id -u)" -eq 0 ]] || { echo "run as root" >&2; exit 1; }
command -v docker >/dev/null || { echo "install Docker first (docs/Setup/VpsBootstrap.md step 1)" >&2; exit 1; }

id "${DEPLOY_USER}" >/dev/null 2>&1 || useradd --system --create-home --shell /bin/bash "${DEPLOY_USER}"
usermod -aG docker "${DEPLOY_USER}"
install -d -m 0755 "${ROOT}"
install -d -m 0700 -o "${DEPLOY_USER}" /etc/cassy
install -d -m 0750 -o "${DEPLOY_USER}" /var/lib/cassy /var/lib/cassy/deploy /var/log/cassy

checkout() {
  local repo="$1" dir="$2"
  if [[ ! -d "${dir}/.git" ]]; then
    git clone --quiet "${ORG}/${repo}.git" "${dir}"
  fi
  git -C "${dir}" fetch --quiet --tags origin
  git -C "${dir}" -c advice.detachedHead=false checkout --quiet "refs/tags/${TAG}"
  # The deploy user runs the scripts but cannot change them.
  chown -R root:root "${dir}"
  chmod -R go-w "${dir}"
}
checkout cassy-backend "${ROOT}/CassyBackend"
checkout cassy-docker "${ROOT}/Docker"

# Compose reads Core/.env; it points at the secrets outside the checkout (Deploy.md).
ln -sfn /etc/cassy/prod.env "${ROOT}/Docker/Core/.env"
for f in prod.env backup.env; do
  if [[ ! -f "/etc/cassy/${f}" ]]; then
    echo "[BOOTSTRAP] /etc/cassy/${f} is missing: create it (mode 0600, owner ${DEPLOY_USER}) per VpsBootstrap.md step 3"
  fi
done
install -m 0644 "${ROOT}/Docker/Backup/CassyBackup.cron" /etc/cron.d/cassy-backup

echo "[BOOTSTRAP] ${ROOT} pinned to ${TAG}: backend $(git -C "${ROOT}/CassyBackend" rev-parse --short HEAD), docker $(git -C "${ROOT}/Docker" rev-parse --short HEAD)"
