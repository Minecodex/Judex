#!/usr/bin/env bash
# Creates the operator-selected external PostgreSQL role/database; no password in SQL text.
set -euo pipefail
PG_HOST="${1:?host}"; PG_PORT="${2:?port}"; ADMIN_USER="${3:?admin user}"; ADMIN_PASSWORD="${4:?admin password}"
JUDEX_PASSWORD="${JUDEX_DB_PASSWORD:?set JUDEX_DB_PASSWORD}"
PGPASSWORD="${ADMIN_PASSWORD}" psql -h "${PG_HOST}" -p "${PG_PORT}" -U "${ADMIN_USER}" -d postgres -v ON_ERROR_STOP=1 -v judex_password="${JUDEX_PASSWORD}" <<'SQL'
SELECT format('CREATE ROLE judex LOGIN PASSWORD %L', :'judex_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='judex')\gexec
SELECT 'CREATE DATABASE judex OWNER judex'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname='judex')\gexec
GRANT ALL PRIVILEGES ON DATABASE judex TO judex;
SQL
printf 'Judex role and database ready.\n'
