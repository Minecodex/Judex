#!/usr/bin/env bash
# Judex 恢复（docs/plans/v1/11 §6）：恢复 PG 到目标库并校验关键链。
# 用法: restore.sh <namespace> <release> <backup-dir>/<cutoff>
set -euo pipefail
NS="${1:?namespace}"; RELEASE="${2:?release}"; SRC="${3:?backup cutoff dir}"
test -f "${SRC}/judex.pgdump" || { echo "missing judex.pgdump"; exit 1; }
echo "==> restore PG"
kubectl -n "${NS}" exec -i "${RELEASE}-postgresql-0" -- \
  pg_restore -U judex -d judex --clean --if-exists < "${SRC}/judex.pgdump"
echo "==> verify"
kubectl -n "${NS}" exec "${RELEASE}-postgresql-0" -- \
  psql -U judex -d judex -tAc "SELECT count(*) FROM users" | { read n; echo "users=${n}"; test "$n" -gt 0; }
kubectl -n "${NS}" exec "${RELEASE}-postgresql-0" -- \
  psql -U judex -d judex -tAc "SELECT count(*) FROM material_versions" | { read n; echo "material_versions=${n}"; }
kubectl -n "${NS}" exec "${RELEASE}-postgresql-0" -- \
  psql -U judex -d judex -tAc "SELECT count(*) FROM proposal_versions WHERE review_hash IS NOT NULL" | { read n; echo "frozen_reviews=${n}"; }
echo "restore verified (材料SHA校验在 S3 对象回放后执行: objects $(wc -l < "${SRC}/objects.txt" 2>/dev/null || echo 0) 行)"
