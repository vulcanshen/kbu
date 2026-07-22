#!/bin/sh
# kbu installer for macOS / Linux / Git Bash
# Usage: curl -fsSL https://raw.githubusercontent.com/vulcanshen/kbu/main/install.sh | sh

set -e

REPO="vulcanshen/kbu"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux*)  OS="linux" ;;
  darwin*) OS="darwin" ;;
  mingw*|msys*|cygwin*) OS="windows" ;;
  *) echo "Error: unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Error: unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest version
echo "Fetching latest release..."
VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed 's/.*"v\(.*\)".*/\1/')
echo "Latest version: $VERSION"

# Set file extension and install dir
if [ "$OS" = "windows" ]; then
  EXT="zip"
  INSTALL_DIR="$HOME/bin"
else
  EXT="tar.gz"
  if [ "$(id -u)" = "0" ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
fi

FILENAME="kbu_${VERSION}_${OS}_${ARCH}.${EXT}"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/v${VERSION}/$FILENAME"

# Download
TMPDIR=$(mktemp -d)
echo "Downloading $FILENAME..."
curl -fsSL "$DOWNLOAD_URL" -o "$TMPDIR/$FILENAME"

# Extract
echo "Extracting..."
if [ "$EXT" = "zip" ]; then
  unzip -o "$TMPDIR/$FILENAME" -d "$TMPDIR" > /dev/null
else
  tar xzf "$TMPDIR/$FILENAME" -C "$TMPDIR"
fi

# Install
mkdir -p "$INSTALL_DIR"
if [ "$OS" = "windows" ]; then
  cp "$TMPDIR/kbu.exe" "$INSTALL_DIR/kbu.exe"
else
  cp "$TMPDIR/kbu" "$INSTALL_DIR/kbu"
  chmod +x "$INSTALL_DIR/kbu"
fi

# Cleanup
rm -rf "$TMPDIR"

echo ""
echo "kbu $VERSION installed to $INSTALL_DIR"

# Check if install dir is in PATH
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo ""
    echo "WARNING: $INSTALL_DIR is not in your PATH."
    echo "Add it by running:"
    echo ""
    if [ "$OS" = "windows" ]; then
      echo "  echo 'export PATH=\"\$HOME/bin:\$PATH\"' >> ~/.bashrc && source ~/.bashrc"
    else
      echo "  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.$(basename "$SHELL")rc && source ~/.$(basename "$SHELL")rc"
    fi
    ;;
esac

echo ""
echo "Run 'kbu --version' to verify."
