#!/usr/bin/env bash
# Restore only to a fresh isolated destination. Objects are checksum verified.
# Usage: restore.sh <new-namespace> <release> <backup-cutoff-directory>
set -euo pipefail
NS="${1:?new namespace}"; RELEASE="${2:?release}"; SRC="${3:?backup directory}"
test -f "${SRC}/manifest.json"
(cd "${SRC}" && sha256sum -c SHA256SUMS)
PG_POD="${JUDEX_BACKUP_PG_POD:-${RELEASE}-postgresql-0}"
# Restore target must be isolated, API-only and empty before replacing its schema.
MODE="$(kubectl -n "${NS}" exec "deployment/${RELEASE}" -- printenv JUDEX_MODE)"
test "${MODE}" = api || { echo "Start the isolated restore deployment in mode api (no workers)." >&2; exit 1; }
USERS="$(kubectl -n "${NS}" exec "${PG_POD}" -- psql -U judex -d judex -Atc 'SELECT count(*) FROM users')"
test "${USERS}" = 0 || { echo "Refusing to restore over a database containing users." >&2; exit 1; }
# The object command refuses a nonempty bucket and verifies the restored SHA256.
kubectl -n "${NS}" exec -i "deployment/${RELEASE}" -- /app/judex-server objects import < "${SRC}/objects.tar"
kubectl -n "${NS}" exec -i "${PG_POD}" -- pg_restore -U judex -d judex --clean --if-exists --exit-on-error < "${SRC}/judex.pgdump"
kubectl -n "${NS}" exec "deployment/${RELEASE}" -- /app/judex-server verify-recovery > "${SRC}/restore-verification.json"
echo "Database restored; all referenced material entry sizes and SHA256 verified. Evidence: ${SRC}/restore-verification.json. Reconcile interrupted runs and pending approvals before enabling workers and traffic."
