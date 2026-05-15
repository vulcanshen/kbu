#!/bin/sh
# km8 uninstaller for macOS / Linux / Git Bash
# Usage: curl -fsSL https://raw.githubusercontent.com/vulcanshen/km8/main/uninstall.sh | sh

set -e

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  mingw*|msys*|cygwin*) OS="windows" ;;
esac

# Determine install locations to check
if [ "$OS" = "windows" ]; then
  CANDIDATES="$HOME/bin/km8.exe"
else
  CANDIDATES="$HOME/.local/bin/km8 /usr/local/bin/km8"
fi

FOUND=""
for path in $CANDIDATES; do
  if [ -f "$path" ]; then
    FOUND="$path"
    break
  fi
done

if [ -z "$FOUND" ]; then
  echo "km8 not found in expected locations."
  echo "Checked: $CANDIDATES"
  exit 1
fi

rm "$FOUND"
echo "removed $FOUND"

# Remove config if present
CONFIG_DIR="$HOME/.config/km8"
if [ -d "$CONFIG_DIR" ]; then
  printf "Remove config in %s? [y/N]: " "$CONFIG_DIR"
  read -r answer
  case "$answer" in
    y|Y|yes|YES)
      rm -rf "$CONFIG_DIR"
      echo "removed $CONFIG_DIR"
      ;;
    *)
      echo "kept $CONFIG_DIR"
      ;;
  esac
fi

echo ""
echo "km8 uninstalled."
