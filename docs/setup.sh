#!/bin/sh
# Interactive setup script for go-postgres-mcp
# Usage: curl -sfL https://peter-trerotola.github.io/go-postgres-mcp/setup.sh | sh
#
# Generates a config.yaml with prompted values.

set -e

CONFIG_FILE="${CONFIG_FILE:-${HOME}/.config/go-postgres-mcp/config.yaml}"

# --- colors (tty-aware) ---

if [ -t 2 ]; then
  BOLD="$(tput bold 2>/dev/null || printf '')"
  DIM="$(tput dim 2>/dev/null || printf '')"
  RED="$(tput setaf 1 2>/dev/null || printf '')"
  GREEN="$(tput setaf 2 2>/dev/null || printf '')"
  YELLOW="$(tput setaf 3 2>/dev/null || printf '')"
  BLUE="$(tput setaf 4 2>/dev/null || printf '')"
  MAGENTA="$(tput setaf 5 2>/dev/null || printf '')"
  RST="$(tput sgr0 2>/dev/null || printf '')"
else
  BOLD='' DIM='' RED='' GREEN='' YELLOW='' BLUE='' MAGENTA='' RST=''
fi

# --- message helpers ---

ohai()      { printf '%s==>%s %s%s\n' "$BLUE" "$BOLD" "$*" "$RST" >&2; }
info()      { printf '   %s\n' "$*" >&2; }
ok()        { printf '   %s✓%s %s\n' "$GREEN" "$RST" "$*" >&2; }
warn()      { printf '   %s!%s %s\n' "$YELLOW" "$RST" "$*" >&2; }
fail()      { printf '   %sx%s %s\n' "$RED" "$RST" "$*" >&2; }

# --- prompts ---

ask() {
  printf '   %s?%s %s ' "$MAGENTA" "$RST" "$1" >&2
  read -r val </dev/tty
  echo "$val"
}

ask_default() {
  printf '   %s?%s %s %s[%s]%s ' "$MAGENTA" "$RST" "$1" "$DIM" "$2" "$RST" >&2
  read -r val </dev/tty
  val="${val:-$2}"
  echo "$val"
}

ask_yn() {
  printf '   %s?%s %s %s[y/n]%s ' "$MAGENTA" "$RST" "$1" "$BOLD" "$RST" >&2
  read -r val </dev/tty
  case "$val" in y|Y|yes|YES) return 0 ;; *) return 1 ;; esac
}

ask_secret() {
  printf '   %s?%s %s ' "$MAGENTA" "$RST" "$1" >&2
  stty -echo 2>/dev/null </dev/tty || true
  set +e
  read -r val </dev/tty
  _read_status=$?
  set -e
  stty echo 2>/dev/null </dev/tty || true
  printf '\n' >&2
  [ "$_read_status" -ne 0 ] && return "$_read_status"
  _label=$1
  case "$_label" in *:) _label=${_label%:} ;; esac
  ok "${_label}: ${DIM}(hidden)${RST}"
  echo "$val"
}

# --- utility helpers ---

collect_databases() {
  info "enter database names, one per line. empty line to finish:"
  _dbs=""
  while true; do
    _v=$(ask "database:")
    [ -z "$_v" ] && break
    if [ -z "$_dbs" ]; then _dbs="$_v"; else _dbs="${_dbs}
${_v}"; fi
  done
  echo "$_dbs"
}

validate_env_name() {
  case "$1" in
    [a-zA-Z_]*) echo "$1" | grep -qE '^[a-zA-Z_][a-zA-Z0-9_]*$' ;;
    *) return 1 ;;
  esac
}

escape_single_quotes() {
  printf '%s' "$1" | sed "s/'/'\\\\''/g"
}

escape_yaml_string() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

expand_tilde() {
  case "$1" in
    "~/"*) printf '%s' "${HOME}/${1#\~/}" ;;
    "~")   printf '%s' "$HOME" ;;
    *)     printf '%s' "$1" ;;
  esac
}

# --- start ---

if [ -z "$GO_POSTGRES_MCP_NO_BANNER" ]; then
  printf '\n' >&2
  printf '%s' "$BLUE" >&2
  cat >&2 <<'BANNER'
   .------------------------------------------------------.
   | what time of day do users sign up most often?          |
   '----.--------------------------------------------------'
        |
     ,_---~~~~~----._                ____  ______  ___
  _,,_,*^____    ___``*g*\"*,      /    )/      \/   \
 / __/ /'    ^. /     \ ^@q  f   (     / __    _\    )
[  @f | @))   || @))   l 0 _/     \    (/ o)  ( o)   )
 \`/   \~___ / _ \____/   \        \_  (_  )   \ )  /
  |          _l_l_         I          \  /\_/    \)_/
  }         [_____]        I           \/  //|  |\
  ]           | | |        |               v |  | v
  ]            ~ ~         |                 \__/
  |                        |
   |                       |
BANNER
  printf '%s' "$RST" >&2
  printf '\n' >&2
fi
ohai "go-postgres-mcp setup"
printf '\n' >&2

CONFIG_DISPLAY=$(echo "$CONFIG_FILE" | sed "s|^${HOME}/|~/|")
CONFIG_FILE=$(ask_default "config file path" "${CONFIG_DISPLAY}")
CONFIG_FILE=$(expand_tilde "$CONFIG_FILE")
ok "config file path: ${BOLD}${CONFIG_FILE}${RST}"
printf '\n' >&2

# --- read-only user check ---

if ! ask_yn "do you have a read-only postgresql user?"; then
  printf '\n' >&2
  info "create one by running the following SQL as a superuser:"
  printf '\n' >&2
  info "  ${BOLD}CREATE ROLE${RST} mcp_reader ${BOLD}WITH LOGIN PASSWORD${RST} 'your-password';"
  info "  ${BOLD}GRANT CONNECT ON DATABASE${RST} your_db ${BOLD}TO${RST} mcp_reader;"
  info "  ${BOLD}GRANT USAGE ON SCHEMA${RST} public ${BOLD}TO${RST} mcp_reader;"
  info "  ${BOLD}GRANT SELECT ON ALL TABLES IN SCHEMA${RST} public ${BOLD}TO${RST} mcp_reader;"
  info "  ${BOLD}ALTER DEFAULT PRIVILEGES IN SCHEMA${RST} public"
  info "    ${BOLD}GRANT SELECT ON TABLES TO${RST} mcp_reader;"
  printf '\n' >&2
  info "for multiple schemas, repeat the GRANT USAGE / GRANT SELECT"
  info "lines for each schema."
  printf '\n' >&2
  if ! ask_yn "ready to continue?"; then
    fail "run this script again when ready."
    exit 0
  fi
  printf '\n' >&2
fi

# --- connection details ---

ohai "Connection"
DB_HOST=$(ask_default "host" "localhost")
ok "host: ${BOLD}${DB_HOST}${RST}"
DB_PORT=$(ask_default "port" "5432")
ok "port: ${BOLD}${DB_PORT}${RST}"
DB_USER=$(ask "user:")
ok "user: ${BOLD}${DB_USER}${RST}"
DB_SSLMODE=$(ask_default "sslmode (disable/require/verify-full)" "require")
ok "sslmode: ${BOLD}${DB_SSLMODE}${RST}"
printf '\n' >&2

# --- password environment variable ---

ohai "Password"
info "the password is read from an environment variable at runtime,"
info "never stored in the config file."
printf '\n' >&2

while true; do
  DB_PASSWORD_ENV=$(ask_default "env var name" "DB_PASSWORD")
  if validate_env_name "$DB_PASSWORD_ENV"; then
    ok "env var name: ${BOLD}${DB_PASSWORD_ENV}${RST}"
    break
  fi
  warn "invalid name — use letters, digits, and underscores only"
done

DB_PASSWORD=""
EXISTING_PASSWORD=$(eval "echo \"\${$DB_PASSWORD_ENV}\"" 2>/dev/null || true)

if [ -n "$EXISTING_PASSWORD" ]; then
  ok "${DB_PASSWORD_ENV} is already set in your environment"
  if ask_yn "use it?"; then
    DB_PASSWORD="$EXISTING_PASSWORD"
  fi
fi

if [ -z "$DB_PASSWORD" ]; then
  if ask_yn "set ${DB_PASSWORD_ENV} now?"; then
    DB_PASSWORD=$(ask_secret "password:")
    SHELL_NAME=$(basename "${SHELL:-/bin/sh}")
    case "$SHELL_NAME" in
      zsh)  RC_FILE="$HOME/.zshrc" ;;
      bash) RC_FILE="$HOME/.bashrc" ;;
      *)    RC_FILE="" ;;
    esac
    ESCAPED_PASSWORD=$(escape_single_quotes "$DB_PASSWORD")
    if [ -n "$RC_FILE" ] && ask_yn "append export to ${RC_FILE}?"; then
      printf '\nexport %s='\''%s'\''\n' "$DB_PASSWORD_ENV" "$ESCAPED_PASSWORD" >> "$RC_FILE"
      ok "added to ${BOLD}${RC_FILE}${RST} — restart your shell or: ${BOLD}source ${RC_FILE}${RST}"
    else
      info "to set in your current shell, run:"
      info "  ${BOLD}export ${DB_PASSWORD_ENV}='${ESCAPED_PASSWORD}'${RST}"
    fi
  fi
fi

printf '\n' >&2

# --- database discovery ---

ohai "Databases"

DATABASES=""

if ask_yn "discover all databases on this host?"; then
  if ! command -v psql >/dev/null 2>&1; then
    warn "psql not found — enter database names manually"
    info "(install postgresql-client to enable auto-discovery)"
    printf '\n' >&2
    DATABASES=$(collect_databases)
  else
    if [ -z "$DB_PASSWORD" ]; then
      DB_PASSWORD=$(ask_secret "password (for discovery, not stored):")
    fi
    info "connecting to ${BOLD}${DB_HOST}:${DB_PORT}${RST}..."
    TMP_ERR=$(mktemp)
    DISCOVERED=$(PGPASSWORD="$DB_PASSWORD" psql \
      -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres \
      --no-psqlrc -t -A \
      -c "SELECT datname FROM pg_database WHERE NOT datistemplate ORDER BY datname" \
      2>"$TMP_ERR") || {
      ERR_MSG=$(cat "$TMP_ERR" 2>/dev/null)
      rm -f "$TMP_ERR"
      fail "connection failed: ${ERR_MSG}"
      printf '\n' >&2
      warn "enter database names manually instead"
      DATABASES=$(collect_databases)
      DISCOVERED=""
    }
    rm -f "$TMP_ERR"
    DB_PASSWORD=""
    if [ -n "$DISCOVERED" ]; then
      DB_COUNT=$(echo "$DISCOVERED" | wc -l | tr -d ' ')
      ok "found ${BOLD}${DB_COUNT}${RST} databases:"
      echo "$DISCOVERED" | while IFS= read -r db; do
        info "  ${GREEN}${db}${RST}"
      done
      printf '\n' >&2
      if ask_yn "use all ${DB_COUNT} databases?"; then
        DATABASES="$DISCOVERED"
        ok "using all ${BOLD}${DB_COUNT}${RST} databases"
      else
        DATABASES=$(collect_databases)
      fi
    fi
  fi
else
  printf '\n' >&2
  DATABASES=$(collect_databases)
fi

if [ -z "$(echo "$DATABASES" | tr -d '[:space:]')" ]; then
  fail "no databases specified"
  exit 1
fi

printf '\n' >&2

# --- per-database filtering ---

CONF_DIR=$(mktemp -d)
trap 'rm -rf "$CONF_DIR"' EXIT INT TERM

DB_IDX=0
echo "$DATABASES" | while IFS= read -r DB; do
  [ -z "$DB" ] && continue
  DB_IDX=$((DB_IDX + 1))
  DB_SCHEMAS=""
  DB_TABLES=""

  if ask_yn "configure filters for ${BOLD}${DB}${RST}? (default: discover all)"; then
    # Schema filter
    if ask_yn "filter schemas for ${DB}?"; then
      info "enter schema names, one per line. empty line to finish:"
      while true; do
        S=$(ask "schema:")
        [ -z "$S" ] && break
        DB_SCHEMAS="${DB_SCHEMAS}
      - \"${S}\""
        ok "schema: ${BOLD}${S}${RST}"
      done
    fi

    # Table filter
    if ask_yn "filter tables for ${DB}?"; then
      FILTER_MODE=$(ask_default "mode" "include")
      ok "mode: ${BOLD}${FILTER_MODE}${RST}"
      info "enter table names (schema.table), one per line. empty line to finish:"
      while true; do
        T=$(ask "table:")
        [ -z "$T" ] && break
        DB_TABLES="${DB_TABLES}
        - \"${T}\""
        ok "table: ${BOLD}${T}${RST}"
      done
      if [ -n "$DB_TABLES" ]; then
        DB_TABLES="
    tables:
      ${FILTER_MODE}:${DB_TABLES}"
      fi
    fi
  else
    ok "${DB}: ${DIM}discover all${RST}"
  fi

  printf '%s' "${DB_SCHEMAS}" > "${CONF_DIR}/${DB_IDX}.schemas"
  printf '%s' "${DB_TABLES}" > "${CONF_DIR}/${DB_IDX}.tables"
done

printf '\n' >&2

# --- knowledge map ---

ohai "Storage"
KM_PATH=$(ask_default "knowledgemap path" "knowledgemap.db")
ok "knowledgemap path: ${BOLD}${KM_PATH}${RST}"

printf '\n' >&2

# --- summary ---

ohai "Summary"
info "${BOLD}host${RST}:         ${GREEN}${DB_HOST}${RST}"
info "${BOLD}port${RST}:         ${GREEN}${DB_PORT}${RST}"
info "${BOLD}user${RST}:         ${GREEN}${DB_USER}${RST}"
info "${BOLD}sslmode${RST}:      ${GREEN}${DB_SSLMODE}${RST}"
info "${BOLD}password env${RST}: ${GREEN}${DB_PASSWORD_ENV}${RST}"
info "${BOLD}databases${RST}:"
echo "$DATABASES" | while IFS= read -r _db; do
  [ -z "$_db" ] && continue
  info "  ${GREEN}${_db}${RST}"
done
info "${BOLD}knowledgemap${RST}: ${GREEN}${KM_PATH}${RST}"
info "${BOLD}config file${RST}:  ${GREEN}${CONFIG_FILE}${RST}"

printf '\n' >&2

# --- write config ---

CONFIG_DIR=$(dirname "$CONFIG_FILE")
if [ ! -d "$CONFIG_DIR" ]; then
  mkdir -p "$CONFIG_DIR"
  ok "created ${BOLD}${CONFIG_DIR}${RST}"
fi

{
  echo "databases:"
  DB_IDX=0
  echo "$DATABASES" | while IFS= read -r DB; do
    [ -z "$DB" ] && continue
    DB_IDX=$((DB_IDX + 1))
    DB_SCHEMAS=$(cat "${CONF_DIR}/${DB_IDX}.schemas" 2>/dev/null || true)
    DB_TABLES=$(cat "${CONF_DIR}/${DB_IDX}.tables" 2>/dev/null || true)
    SAFE_DB=$(escape_yaml_string "$DB")
    cat << ENTRY
  - name: "${SAFE_DB}"
    host: "${DB_HOST}"
    port: ${DB_PORT}
    database: "${SAFE_DB}"
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

printf '\n' >&2
ok "wrote ${BOLD}${CONFIG_FILE}${RST}"
printf '\n' >&2

# --- next steps ---

ohai "Next steps"
CURRENT_PASSWORD=$(eval "echo \"\${$DB_PASSWORD_ENV}\"" 2>/dev/null || true)
if [ -z "$CURRENT_PASSWORD" ]; then
  info "  ${BOLD}export ${DB_PASSWORD_ENV}='your-password'${RST}"
fi
info "  ${BOLD}go-postgres-mcp --config ${CONFIG_FILE}${RST}"
printf '\n' >&2

# --- claude code integration ---

if command -v claude >/dev/null 2>&1; then
  ohai "Claude Code"
  info "detected ${BOLD}claude${RST} CLI"
  if ask_yn "add as an MCP server in Claude Code?"; then
    MCP_NAME=$(ask_default "server name" "postgres")
    ok "server name: ${BOLD}${MCP_NAME}${RST}"

    # Detect binary path
    if command -v go-postgres-mcp >/dev/null 2>&1; then
      MCP_BIN=$(command -v go-postgres-mcp)
    else
      MCP_BIN="go-postgres-mcp"
    fi

    # Pass env var with ${VAR} interpolation so Claude Code reads it
    # from the user's environment at runtime
    info "running: ${DIM}claude mcp add ...${RST}"
    if claude mcp add --transport stdio \
      --env "${DB_PASSWORD_ENV}=\${${DB_PASSWORD_ENV}}" \
      "$MCP_NAME" -- "$MCP_BIN" --config "$CONFIG_FILE"; then
      ok "added ${BOLD}${MCP_NAME}${RST} to Claude Code"
    else
      warn "could not add MCP server automatically"
      info "add it manually:"
      info "  ${BOLD}claude mcp add --transport stdio --env ${DB_PASSWORD_ENV}=\${${DB_PASSWORD_ENV}} ${MCP_NAME} -- ${MCP_BIN} --config ${CONFIG_FILE}${RST}"
    fi
    printf '\n' >&2
  fi
fi
