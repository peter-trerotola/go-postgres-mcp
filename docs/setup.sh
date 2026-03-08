#!/bin/sh
# Interactive setup script for go-postgres-mcp
# Usage: curl -sfL https://peter-trerotola.github.io/go-postgres-mcp/setup.sh | sh
#
# Generates a config.yaml with prompted values.

set -e

CONFIG_FILE="${CONFIG_FILE:-config.yaml}"

# --- helpers ---

prompt() {
  printf '%s' "$1" >&2
  read -r val </dev/tty
  echo "$val"
}

prompt_default() {
  printf '%s [%s] ' "$1" "$2" >&2
  read -r val </dev/tty
  if [ -z "$val" ]; then echo "$2"; else echo "$val"; fi
}

prompt_yn() {
  printf '%s [y/n] ' "$1" >&2
  read -r val </dev/tty
  case "$val" in y|Y|yes|YES) return 0 ;; *) return 1 ;; esac
}

prompt_secret() {
  printf '%s' "$1" >&2
  stty -echo 2>/dev/null </dev/tty || true
  read -r val </dev/tty
  stty echo 2>/dev/null </dev/tty || true
  printf '\n' >&2
  echo "$val"
}

# collect_list "prompt" — reads lines until empty, returns space-separated
collect_list() {
  _items=""
  while true; do
    _v=$(prompt "$1")
    [ -z "$_v" ] && break
    _items="${_items} ${_v}"
  done
  echo "$_items"
}

echo ""
echo "go-postgres-mcp setup"
echo "====================="
echo ""

# --- read-only user check ---

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

# --- connection details ---

DB_HOST=$(prompt_default "host" "localhost")
DB_PORT=$(prompt_default "port" "5432")
DB_USER=$(prompt "user: ")
echo ""  >&2
echo "  the password is read from an environment variable at runtime," >&2
echo "  never stored in the config file." >&2
echo "" >&2
DB_PASSWORD_ENV=$(prompt_default "env var name for the password" "DB_PASSWORD")
DB_SSLMODE=$(prompt_default "sslmode (disable/require/verify-full)" "disable")

echo ""

# --- set up environment variable ---

if prompt_yn "set ${DB_PASSWORD_ENV} now?"; then
  DB_PASSWORD=$(prompt_secret "  password: ")
  SHELL_NAME=$(basename "${SHELL:-/bin/sh}")
  case "$SHELL_NAME" in
    zsh)  RC_FILE="$HOME/.zshrc" ;;
    bash) RC_FILE="$HOME/.bashrc" ;;
    *)    RC_FILE="" ;;
  esac
  if [ -n "$RC_FILE" ] && prompt_yn "  append export to ${RC_FILE}?"; then
    printf '\nexport %s='\''%s'\''\n' "$DB_PASSWORD_ENV" "$DB_PASSWORD" >> "$RC_FILE"
    echo "  added to ${RC_FILE} — restart your shell or run: source ${RC_FILE}" >&2
  else
    export "${DB_PASSWORD_ENV}=${DB_PASSWORD}"
    echo "  exported ${DB_PASSWORD_ENV} for this session" >&2
  fi
  echo ""
fi

# --- database discovery ---

DATABASES=""

if prompt_yn "discover all databases on this host?"; then
  echo ""
  if ! command -v psql >/dev/null 2>&1; then
    echo "  psql not found — enter database names manually." >&2
    echo "  (install postgresql-client to enable auto-discovery)" >&2
    echo ""
    echo "  enter database names, one per line. empty line to finish:" >&2
    DATABASES=$(collect_list "  database: ")
  else
    # use existing env var or prompt for password
    if [ -n "$DB_PASSWORD" ]; then
      PASS="$DB_PASSWORD"
    else
      PASS=$(prompt_secret "  password (for discovery, not stored): ")
    fi
    echo "  connecting to ${DB_HOST}:${DB_PORT}..." >&2
    DISCOVERED=$(PGPASSWORD="$PASS" psql \
      -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres \
      -t -A -c "SELECT datname FROM pg_database WHERE NOT datistemplate ORDER BY datname" 2>&1) || {
      echo "  connection failed: ${DISCOVERED}" >&2
      echo "" >&2
      echo "  enter database names manually instead." >&2
      echo "  enter database names, one per line. empty line to finish:" >&2
      DATABASES=$(collect_list "  database: ")
      DISCOVERED=""
    }
    if [ -n "$DISCOVERED" ]; then
      DB_COUNT=$(echo "$DISCOVERED" | wc -l | tr -d ' ')
      echo "  found ${DB_COUNT} databases:" >&2
      echo "$DISCOVERED" | while IFS= read -r db; do
        echo "    - ${db}" >&2
      done
      echo "" >&2
      if prompt_yn "  use all ${DB_COUNT} databases?"; then
        DATABASES=$(echo "$DISCOVERED" | tr '\n' ' ')
      else
        echo "  enter the databases you want, one per line. empty line to finish:" >&2
        DATABASES=$(collect_list "  database: ")
      fi
    fi
  fi
else
  echo ""
  echo "  enter database names, one per line. empty line to finish:" >&2
  DATABASES=$(collect_list "  database: ")
fi

if [ -z "$(echo "$DATABASES" | tr -d ' ')" ]; then
  echo "  no databases specified. exiting." >&2
  exit 1
fi

echo ""

# --- per-database filtering ---

# DB_CONFIGS holds per-database YAML fragments (schema/table filters)
# indexed by database name via temp files
CONF_DIR=$(mktemp -d)
trap "rm -rf '$CONF_DIR'" EXIT INT TERM

DB_COUNT=0
for DB in $DATABASES; do
  DB_COUNT=$((DB_COUNT + 1))
done

if [ "$DB_COUNT" -gt 1 ]; then
  echo "  configuring ${DB_COUNT} databases" >&2
fi

for DB in $DATABASES; do
  DB_SCHEMAS=""
  DB_TABLES=""

  if prompt_yn "configure filters for ${DB}? (default: discover all)"; then
    # Schema filter
    if prompt_yn "  filter schemas for ${DB}?"; then
      echo "    enter schema names, one per line. empty line to finish:" >&2
      while true; do
        S=$(prompt "    schema: ")
        [ -z "$S" ] && break
        DB_SCHEMAS="${DB_SCHEMAS}
      - \"${S}\""
      done
    fi

    # Table filter
    if prompt_yn "  filter tables for ${DB}?"; then
      FILTER_MODE=$(prompt_default "    mode (include/exclude)" "include")
      echo "    enter table names (schema.table), one per line. empty line to finish:" >&2
      while true; do
        T=$(prompt "    table: ")
        [ -z "$T" ] && break
        DB_TABLES="${DB_TABLES}
        - \"${T}\""
      done
      if [ -n "$DB_TABLES" ]; then
        DB_TABLES="
    tables:
      ${FILTER_MODE}:${DB_TABLES}"
      fi
    fi
  fi

  # write per-db fragment
  printf '%s' "${DB_SCHEMAS}" > "${CONF_DIR}/${DB}.schemas"
  printf '%s' "${DB_TABLES}" > "${CONF_DIR}/${DB}.tables"
done

echo ""

# --- knowledge map ---

KM_PATH=$(prompt_default "knowledgemap path" "knowledgemap.db")

# --- write config ---

{
  echo "databases:"
  for DB in $DATABASES; do
    DB_SCHEMAS=$(cat "${CONF_DIR}/${DB}.schemas" 2>/dev/null || true)
    DB_TABLES=$(cat "${CONF_DIR}/${DB}.tables" 2>/dev/null || true)
    cat << ENTRY
  - name: "${DB}"
    host: "${DB_HOST}"
    port: ${DB_PORT}
    database: "${DB}"
    user: "${DB_USER}"
    password_env: "${DB_PASSWORD_ENV}"
    sslmode: "${DB_SSLMODE}"${DB_SCHEMAS:+
    schemas:${DB_SCHEMAS}}${DB_TABLES}
ENTRY
  done
  cat << FOOTER

knowledgemap:
  path: "${KM_PATH}"
  auto_discover_on_startup: true
FOOTER
} > "$CONFIG_FILE"

echo ""
echo "wrote ${CONFIG_FILE}"
echo ""
echo "next steps:"
if [ -z "$DB_PASSWORD" ]; then
  echo "  export ${DB_PASSWORD_ENV}='your-password'"
fi
echo "  go-postgres-mcp --config ${CONFIG_FILE}"
