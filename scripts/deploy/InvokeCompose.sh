#!/usr/bin/env bash
# docker compose for production with the digests DeployRelease.sh recorded, e.g. `InvokeCompose.sh logs api`
# or `InvokeCompose.sh up -d alloy`. Deploy new images with DeployRelease.sh, not with this wrapper.
set -euo pipefail
set -a
. "${DEPLOY_STATE_DIR:-/var/lib/cassy/deploy}/current.env"
set +a
[[ -n "${WEB_IMAGE_DIGEST:-}" ]] && export COMPOSE_PROFILES=web
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_FILE="${DEPLOY_COMPOSE_FILE:-$(cd "${REPO_DIR}/../Docker" 2>/dev/null && pwd)/Core/DockerComposeProd.yml}"
exec docker compose -f "${COMPOSE_FILE}" ${DEPLOY_COMPOSE_OVERRIDE:+-f "${DEPLOY_COMPOSE_OVERRIDE}"} "$@"
