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
  set +e
  read -r val </dev/tty
  _read_status=$?
  set -e
  stty echo 2>/dev/null </dev/tty || true
  printf '\n' >&2
  [ "$_read_status" -ne 0 ] && return "$_read_status"
  echo "$val"
}

# collect_databases — prompts for database names, one per line (newline-delimited)
collect_databases() {
  echo "  enter database names, one per line. empty line to finish:" >&2
  _dbs=""
  while true; do
    _v=$(prompt "  database: ")
    [ -z "$_v" ] && break
    if [ -z "$_dbs" ]; then
      _dbs="$_v"
    else
      _dbs="$_dbs
$_v"
    fi
  done
  echo "$_dbs"
}

# validate_env_name — checks that a string is a valid env var name
validate_env_name() {
  case "$1" in
    [a-zA-Z_]*) echo "$1" | grep -qE '^[a-zA-Z_][a-zA-Z0-9_]*$' ;;
    *) return 1 ;;
  esac
}

# escape_single_quotes — replaces ' with '\'' for safe shell quoting
escape_single_quotes() {
  printf '%s' "$1" | sed "s/'/'\\\\''/g"
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
while true; do
  DB_PASSWORD_ENV=$(prompt_default "env var name for the password" "DB_PASSWORD")
  if validate_env_name "$DB_PASSWORD_ENV"; then
    break
  fi
  echo "  invalid env var name — use letters, digits, and underscores only" >&2
done
DB_SSLMODE=$(prompt_default "sslmode (disable/require/verify-full)" "require")

echo ""

# --- set up environment variable ---

DB_PASSWORD=""
EXISTING_PASSWORD=$(eval "echo \"\${$DB_PASSWORD_ENV}\"" 2>/dev/null || true)

if [ -n "$EXISTING_PASSWORD" ]; then
  echo "  ${DB_PASSWORD_ENV} is already set in your environment." >&2
  DB_PASSWORD="$EXISTING_PASSWORD"
elif prompt_yn "set ${DB_PASSWORD_ENV} now?"; then
  DB_PASSWORD=$(prompt_secret "  password: ")
  SHELL_NAME=$(basename "${SHELL:-/bin/sh}")
  case "$SHELL_NAME" in
    zsh)  RC_FILE="$HOME/.zshrc" ;;
    bash) RC_FILE="$HOME/.bashrc" ;;
    *)    RC_FILE="" ;;
  esac
  ESCAPED_PASSWORD=$(escape_single_quotes "$DB_PASSWORD")
  if [ -n "$RC_FILE" ] && prompt_yn "  append export to ${RC_FILE}?"; then
    printf '\nexport %s='\''%s'\''\n' "$DB_PASSWORD_ENV" "$ESCAPED_PASSWORD" >> "$RC_FILE"
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
    DATABASES=$(collect_databases)
  else
    # use existing password or prompt for one
    if [ -n "$DB_PASSWORD" ]; then
      PASS="$DB_PASSWORD"
    else
      PASS=$(prompt_secret "  password (for discovery, not stored): ")
    fi
    echo "  connecting to ${DB_HOST}:${DB_PORT}..." >&2
    TMP_ERR=$(mktemp)
    DISCOVERED=$(PGPASSWORD="$PASS" psql \
      -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres \
      --no-psqlrc -t -A \
      -c "SELECT datname FROM pg_database WHERE NOT datistemplate ORDER BY datname" \
      2>"$TMP_ERR") || {
      ERR_MSG=$(cat "$TMP_ERR" 2>/dev/null)
      rm -f "$TMP_ERR"
      echo "  connection failed: ${ERR_MSG}" >&2
      echo "" >&2
      echo "  enter database names manually instead." >&2
      DATABASES=$(collect_databases)
      DISCOVERED=""
    }
    rm -f "$TMP_ERR"
    PASS=""
    if [ -n "$DISCOVERED" ]; then
      DB_COUNT=$(echo "$DISCOVERED" | wc -l | tr -d ' ')
      echo "  found ${DB_COUNT} databases:" >&2
      echo "$DISCOVERED" | while IFS= read -r db; do
        echo "    - ${db}" >&2
      done
      echo "" >&2
      if prompt_yn "  use all ${DB_COUNT} databases?"; then
        DATABASES="$DISCOVERED"
      else
        DATABASES=$(collect_databases)
      fi
    fi
  fi
else
  echo ""
  DATABASES=$(collect_databases)
fi

if [ -z "$(echo "$DATABASES" | tr -d '[:space:]')" ]; then
  echo "  no databases specified. exiting." >&2
  exit 1
fi

echo ""

# --- per-database filtering ---

CONF_DIR=$(mktemp -d)
trap "rm -rf '$CONF_DIR'" EXIT INT TERM

DB_COUNT=0
echo "$DATABASES" | while IFS= read -r _d; do
  [ -z "$_d" ] && continue
  DB_COUNT=$((DB_COUNT + 1))
done

DB_IDX=0
echo "$DATABASES" | while IFS= read -r DB; do
  [ -z "$DB" ] && continue
  DB_IDX=$((DB_IDX + 1))
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

  # write per-db fragment using numeric index to avoid path traversal
  printf '%s' "${DB_SCHEMAS}" > "${CONF_DIR}/${DB_IDX}.schemas"
  printf '%s' "${DB_TABLES}" > "${CONF_DIR}/${DB_IDX}.tables"
done

echo ""

# --- knowledge map ---

KM_PATH=$(prompt_default "knowledgemap path" "knowledgemap.db")

# --- write config ---

{
  echo "databases:"
  DB_IDX=0
  echo "$DATABASES" | while IFS= read -r DB; do
    [ -z "$DB" ] && continue
    DB_IDX=$((DB_IDX + 1))
    DB_SCHEMAS=$(cat "${CONF_DIR}/${DB_IDX}.schemas" 2>/dev/null || true)
    DB_TABLES=$(cat "${CONF_DIR}/${DB_IDX}.tables" 2>/dev/null || true)
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
CURRENT_PASSWORD=$(eval "echo \"\${$DB_PASSWORD_ENV}\"" 2>/dev/null || true)
if [ -z "$CURRENT_PASSWORD" ]; then
  echo "  export ${DB_PASSWORD_ENV}='your-password'"
fi
echo "  go-postgres-mcp --config ${CONFIG_FILE}"
