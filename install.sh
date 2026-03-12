#!/bin/sh
# Install script for skillreg
# Usage: curl -sSL https://raw.githubusercontent.com/kochkarovv/skillreg/main/install.sh | sh

set -e

REPO="kochkarovv/skillreg"

# Detect OS
OS="$(uname -s)"
case "$OS" in
    Linux*)  OS=linux;;
    Darwin*) OS=darwin;;
    *)       echo "Unsupported OS: $OS"; exit 1;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64)  ARCH=amd64;;
    aarch64) ARCH=arm64;;
    arm64)   ARCH=arm64;;
    *)       echo "Unsupported architecture: $ARCH"; exit 1;;
esac

echo "Detected platform: ${OS}/${ARCH}"

# Get latest release tag
echo "Fetching latest release..."
TAG=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -z "$TAG" ]; then
    echo "Error: could not determine latest release"
    exit 1
fi

echo "Latest version: ${TAG}"

# Download
URL="https://github.com/${REPO}/releases/download/${TAG}/skillreg_${OS}_${ARCH}.tar.gz"
TMPDIR=$(mktemp -d)
ARCHIVE="${TMPDIR}/skillreg.tar.gz"

echo "Downloading ${URL}..."
curl -sSL -o "$ARCHIVE" "$URL"

# Extract
echo "Extracting..."
tar -xzf "$ARCHIVE" -C "$TMPDIR"

# Install
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
    INSTALL_DIR="${HOME}/.local/bin"
    mkdir -p "$INSTALL_DIR"
    echo "No write access to /usr/local/bin, installing to ${INSTALL_DIR}"
fi

mv "${TMPDIR}/skillreg" "${INSTALL_DIR}/skillreg"
chmod +x "${INSTALL_DIR}/skillreg"

# Cleanup
rm -rf "$TMPDIR"

# Verify
if command -v skillreg >/dev/null 2>&1; then
    echo "Installed successfully: $(skillreg --version)"
else
    echo "Installed to ${INSTALL_DIR}/skillreg"
    echo "Make sure ${INSTALL_DIR} is in your PATH"
fi
