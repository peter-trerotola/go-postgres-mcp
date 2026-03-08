#!/bin/sh
# Install script for go-postgres-mcp
# Usage: curl -sfL https://peter-trerotola.github.io/go-postgres-mcp/install.sh | sh
#
# Environment variables:
#   VERSION      - specific version to install (default: latest)
#   INSTALL_DIR  - installation directory (default: /usr/local/bin)

set -e

REPO="peter-trerotola/go-postgres-mcp"
BINARY="go-postgres-mcp"
SETUP_URL="https://peter-trerotola.github.io/go-postgres-mcp/setup.sh"

# --- colors (tty-aware) ---

if [ -t 2 ]; then
  BOLD="$(tput bold 2>/dev/null || printf '')"
  DIM="$(tput setaf 0 2>/dev/null || printf '')"
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

ohai()      { printf "${BLUE}==>${BOLD} %s${RST}\n" "$*" >&2; }
info()      { printf "   %s\n" "$*" >&2; }
ok()        { printf "   ${GREEN}✓${RST} %s\n" "$*" >&2; }
warn()      { printf "   ${YELLOW}!${RST} %s\n" "$*" >&2; }
fail()      { printf "   ${RED}x${RST} %s\n" "$*" >&2; exit 1; }

ask_yn() {
  printf "   ${MAGENTA}?${RST} %s ${BOLD}[y/n]${RST} " "$1" >&2
  read -r val </dev/tty
  case "$val" in y|Y|yes|YES) return 0 ;; *) return 1 ;; esac
}

# --- detect platform ---

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux|darwin) ;;
  *) fail "unsupported OS: ${OS}" ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) fail "unsupported architecture: ${ARCH}" ;;
esac

# --- resolve version ---

if [ -z "$VERSION" ]; then
  VERSION=$(curl -sfL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
  [ -z "$VERSION" ] && fail "could not determine latest version"
fi

VERSION_NUM="${VERSION#v}"

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
TARBALL="${BINARY}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

# --- install ---

printf '\n' >&2
printf "${BLUE}" >&2
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
printf "${RST}" >&2
printf '\n' >&2
ohai "Installing ${BINARY} ${VERSION}"
info "${BOLD}platform${RST}:  ${GREEN}${OS}/${ARCH}${RST}"
info "${BOLD}install${RST}:   ${GREEN}${INSTALL_DIR}${RST}"
printf '\n' >&2

# Create temp dir with cleanup
TMP_DIR=$(mktemp -d)
trap "rm -rf '$TMP_DIR'" EXIT INT TERM

# Download
info "downloading ${DIM}${TARBALL}${RST}..."
curl -sfL -o "${TMP_DIR}/${TARBALL}" "${BASE_URL}/${TARBALL}" || fail "download failed — check that ${VERSION} exists for ${OS}/${ARCH}"
curl -sfL -o "${TMP_DIR}/checksums.txt" "${BASE_URL}/checksums.txt" || fail "could not download checksums"
ok "downloaded"

# Verify checksum
info "verifying checksum..."
EXPECTED=$(grep "${TARBALL}" "${TMP_DIR}/checksums.txt" | awk '{print $1}')
[ -z "$EXPECTED" ] && fail "no checksum found for ${TARBALL}"

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "${TMP_DIR}/${TARBALL}" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "${TMP_DIR}/${TARBALL}" | awk '{print $1}')
elif command -v openssl >/dev/null 2>&1; then
  ACTUAL=$(openssl dgst -sha256 "${TMP_DIR}/${TARBALL}" | awk '{print $NF}')
else
  fail "no SHA256 tool found (need sha256sum, shasum, or openssl)"
fi

if [ "$EXPECTED" != "$ACTUAL" ]; then
  fail "checksum mismatch — expected: ${EXPECTED}, actual: ${ACTUAL}"
fi
ok "checksum verified"

# Extract
info "extracting..."
tar -xzf "${TMP_DIR}/${TARBALL}" -C "${TMP_DIR}"
ok "extracted"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  warn "writing to ${INSTALL_DIR} requires elevated permissions"
  sudo mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi
chmod +x "${INSTALL_DIR}/${BINARY}"

printf '\n' >&2
ok "installed ${BOLD}${BINARY} ${VERSION}${RST} to ${GREEN}${INSTALL_DIR}/${BINARY}${RST}"
printf '\n' >&2

# --- offer to run setup ---

if ask_yn "run the configuration wizard now?"; then
  printf '\n' >&2
  curl -sfL "$SETUP_URL" | GO_POSTGRES_MCP_NO_BANNER=1 sh
else
  ohai "Next steps"
  info "  ${BOLD}${BINARY} --help${RST}"
  info "  ${BOLD}curl -sfL ${SETUP_URL} | sh${RST}"
  printf '\n' >&2
fi
