#!/usr/bin/env bash
# Judex 备份（docs/plans/v1/11 §6）：PG 自定义格式 + S3 对象清单 + 版本 manifest。
# 用法: backup.sh <namespace> <release> <backup-dir>
set -euo pipefail
NS="${1:?namespace}"; RELEASE="${2:?release}"; OUT="${3:?backup-dir}"
CUTOFF="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "${OUT}/${CUTOFF}"
echo "==> PG dump"
kubectl -n "${NS}" exec "${RELEASE}-postgresql-0" -- \
  pg_dump -U judex -Fc -d judex > "${OUT}/${CUTOFF}/judex.pgdump"
echo "==> S3 object inventory"
kubectl -n "${NS}" exec "${RELEASE}-seaweedfs-0" -- \
  sh -c "mc alias set local http://127.0.0.1:8333 \$(cat /run/secrets/storage/access-key) \$(cat /run/secrets/storage/secret-key) 2>/dev/null; mc ls --recursive local/judex" \
  > "${OUT}/${CUTOFF}/objects.txt" || echo "(inventory best-effort)"
cat > "${OUT}/${CUTOFF}/manifest.json" <<EOF
{"cutoff":"${CUTOFF}","namespace":"${NS}","release":"${RELEASE}","pg":"judex.pgdump","objects":"objects.txt"}
EOF
echo "backup complete: ${OUT}/${CUTOFF}"
