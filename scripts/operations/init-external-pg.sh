#!/usr/bin/env bash
# 外部 PG 预建脚本（P7 部署前置条件）：在使用外部 PostgreSQL 时，
# 必须先创建 judex 用户和数据库。用法: init-external-pg.sh <pg-host> <pg-port> <admin-user> <admin-password>
set -euo pipefail
HOST="${1:?host}"; PORT="${2:?port}"; ADMIN="${3:?admin user}"; PASS="${4:?admin password}"
JUDEX_PASS="${JUDEX_DB_PASSWORD:?set JUDEX_DB_PASSWORD}"
psql -h "$HOST" -p "$PORT" -U "$ADMIN" -d postgres <<SQL
DO \$\$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'judex') THEN
    CREATE ROLE judex LOGIN PASSWORD '$JUDEX_PASS';
  END IF;
END \$\$;
SELECT 'CREATE DATABASE judex OWNER judex'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'judex')\gexec
GRANT ALL PRIVILEGES ON DATABASE judex TO judex;
SQL
echo "✓ judex user + database created"
