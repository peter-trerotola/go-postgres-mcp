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

log() { echo "  $1"; }
fail() { printf 'ERROR: %s\n' "$1" >&2; exit 1; }

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux|darwin) ;;
  *) fail "Unsupported OS: $OS" ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) fail "Unsupported architecture: $ARCH" ;;
esac

# Resolve version
if [ -z "$VERSION" ]; then
  VERSION=$(curl -sfL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
  [ -z "$VERSION" ] && fail "Could not determine latest version"
fi

# Strip leading v for filename
VERSION_NUM="${VERSION#v}"

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
TARBALL="${BINARY}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

echo "Installing ${BINARY} ${VERSION} (${OS}/${ARCH})"

# Create temp dir with cleanup
TMP_DIR=$(mktemp -d)
trap "rm -rf '$TMP_DIR'" EXIT INT TERM

# Download tarball and checksums
log "Downloading ${TARBALL}..."
curl -sfL -o "${TMP_DIR}/${TARBALL}" "${BASE_URL}/${TARBALL}" || fail "Download failed. Check that ${VERSION} exists for ${OS}/${ARCH}."
curl -sfL -o "${TMP_DIR}/checksums.txt" "${BASE_URL}/checksums.txt" || fail "Could not download checksums"

# Verify checksum
log "Verifying checksum..."
EXPECTED=$(grep "${TARBALL}" "${TMP_DIR}/checksums.txt" | awk '{print $1}')
[ -z "$EXPECTED" ] && fail "No checksum found for ${TARBALL}"

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "${TMP_DIR}/${TARBALL}" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "${TMP_DIR}/${TARBALL}" | awk '{print $1}')
elif command -v openssl >/dev/null 2>&1; then
  ACTUAL=$(openssl dgst -sha256 "${TMP_DIR}/${TARBALL}" | awk '{print $NF}')
else
  fail "No SHA256 tool found (need sha256sum, shasum, or openssl)"
fi

if [ "$EXPECTED" != "$ACTUAL" ]; then
  printf 'ERROR: Checksum mismatch\n  expected: %s\n  actual:   %s\n' "$EXPECTED" "$ACTUAL" >&2
  exit 1
fi

# Extract
log "Extracting..."
tar -xzf "${TMP_DIR}/${TARBALL}" -C "${TMP_DIR}"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  log "Writing to ${INSTALL_DIR} requires elevated permissions"
  sudo mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi
chmod +x "${INSTALL_DIR}/${BINARY}"

echo ""
echo "Installed ${BINARY} ${VERSION} to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "Quick start:"
echo "  ${BINARY} --help"
