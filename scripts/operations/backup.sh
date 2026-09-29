#!/usr/bin/env bash
# Immutable object bytes are captured after the database snapshot.
# Usage: backup.sh <namespace> <release> <backup-directory>
set -euo pipefail
umask 077
NS="${1:?namespace}"; RELEASE="${2:?release}"; OUT="${3:?backup-directory}"
CUTOFF="$(date -u +%Y%m%dT%H%M%SZ)"
DEST="${OUT}/${CUTOFF}"
mkdir -p "${DEST}"
PG_POD="${JUDEX_BACKUP_PG_POD:-${RELEASE}-postgresql-0}"
kubectl -n "${NS}" exec "${PG_POD}" -- pg_dump -U judex -Fc -d judex > "${DEST}/judex.pgdump.partial"
mv "${DEST}/judex.pgdump.partial" "${DEST}/judex.pgdump"
kubectl -n "${NS}" exec "deployment/${RELEASE}" -- /app/judex-server objects export > "${DEST}/objects.tar.partial"
mv "${DEST}/objects.tar.partial" "${DEST}/objects.tar"
(cd "${DEST}" && sha256sum judex.pgdump objects.tar > SHA256SUMS)
printf '{"format":1,"cutoff":"%s","pg":"judex.pgdump","objects":"objects.tar","checksums":"SHA256SUMS"}\n' "${CUTOFF}" > "${DEST}/manifest.json"
echo "Backup complete: ${DEST}"
