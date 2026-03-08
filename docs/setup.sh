#!/bin/sh
# Interactive setup script for go-postgres-mcp
# Usage: curl -sfL https://peter-trerotola.github.io/go-postgres-mcp/setup.sh | sh
#
# Generates a config.yaml with prompted values.

set -e

CONFIG_FILE="${CONFIG_FILE:-config.yaml}"

prompt() {
  printf '%s' "$1" >&2
  read -r val
  echo "$val"
}

prompt_default() {
  printf '%s [%s] ' "$1" "$2" >&2
  read -r val
  if [ -z "$val" ]; then echo "$2"; else echo "$val"; fi
}

prompt_yn() {
  printf '%s [y/n] ' "$1" >&2
  read -r val
  case "$val" in y|Y|yes|YES) return 0 ;; *) return 1 ;; esac
}

echo ""
echo "go-postgres-mcp setup"
echo "====================="
echo ""

# Check for read-only user
if ! prompt_yn "do you have a read-only postgresql user?"; then
  echo ""
  echo "  create one by running the following SQL as a superuser:"
  echo ""
  echo "    CREATE ROLE mcp_reader WITH LOGIN PASSWORD 'your-password';"
  echo "    GRANT CONNECT ON DATABASE your_db TO mcp_reader;"
  echo "    GRANT USAGE ON SCHEMA public TO mcp_reader;"
  echo "    GRANT SELECT ON ALL TABLES IN SCHEMA public TO mcp_reader;"
  echo "    ALTER DEFAULT PRIVILEGES IN SCHEMA public"
  echo "      GRANT SELECT ON TABLES TO mcp_reader;"
  echo ""
  echo "  for multiple schemas, repeat the GRANT USAGE / GRANT SELECT"
  echo "  lines for each schema."
  echo ""
  echo "  the ALTER DEFAULT PRIVILEGES line ensures future tables are"
  echo "  also readable. adjust the schema and role name as needed."
  echo ""
  if ! prompt_yn "ready to continue?"; then
    echo "  run this script again when ready." >&2
    exit 0
  fi
  echo ""
fi

# Database config
DB_NAME=$(prompt_default "database name (label)" "mydb")
DB_HOST=$(prompt_default "host" "localhost")
DB_PORT=$(prompt_default "port" "5432")
DB_DATABASE=$(prompt "database: ")
DB_USER=$(prompt "user: ")
DB_PASSWORD_ENV=$(prompt_default "password env var name" "DB_PASSWORD")
DB_SSLMODE=$(prompt_default "sslmode (disable/require/verify-full)" "disable")

# Schema filter
SCHEMAS=""
if prompt_yn "filter schemas? (default: discover all)"; then
  echo "  enter schema names, one per line. empty line to finish:" >&2
  while true; do
    S=$(prompt "  schema: ")
    [ -z "$S" ] && break
    SCHEMAS="${SCHEMAS}
      - \"${S}\""
  done
fi

# Table filter
TABLE_FILTER=""
if prompt_yn "filter tables? (default: discover all)"; then
  FILTER_MODE=$(prompt_default "  mode (include/exclude)" "include")
  echo "  enter table names (schema.table), one per line. empty line to finish:" >&2
  while true; do
    T=$(prompt "  table: ")
    [ -z "$T" ] && break
    TABLE_FILTER="${TABLE_FILTER}
        - \"${T}\""
  done
  if [ -n "$TABLE_FILTER" ]; then
    TABLE_FILTER="
    tables:
      ${FILTER_MODE}:${TABLE_FILTER}"
  fi
fi

# Knowledge map
KM_PATH=$(prompt_default "knowledgemap path" "knowledgemap.db")

# Write config
cat > "$CONFIG_FILE" << YAML
databases:
  - name: "${DB_NAME}"
    host: "${DB_HOST}"
    port: ${DB_PORT}
    database: "${DB_DATABASE}"
    user: "${DB_USER}"
    password_env: "${DB_PASSWORD_ENV}"
    sslmode: "${DB_SSLMODE}"${SCHEMAS:+
    schemas:${SCHEMAS}}${TABLE_FILTER}

knowledgemap:
  path: "${KM_PATH}"
  auto_discover_on_startup: true
YAML

echo ""
echo "wrote ${CONFIG_FILE}"
echo ""
echo "next steps:"
echo "  export ${DB_PASSWORD_ENV}='your-password'"
echo "  go-postgres-mcp --config ${CONFIG_FILE}"
