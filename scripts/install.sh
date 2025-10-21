#!/bin/bash
set -e

# ore-light installer script
# Usage: curl -Ls https://raw.githubusercontent.com/contriboss/ore-light/main/scripts/install.sh | bash

REPO="contriboss/ore-light"
INSTALL_DIR="${ORE_INSTALL_DIR:-/usr/local/bin}"

echo "Installing ore-light..."

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  linux)
    OS="linux"
    ;;
  darwin)
    OS="darwin"
    ;;
  mingw*|msys*|cygwin*)
    OS="windows"
    ;;
  *)
    echo "Unsupported operating system: $OS"
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64|amd64)
    ARCH="amd64"
    ;;
  aarch64|arm64)
    ARCH="arm64"
    ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

BINARY_NAME="ore-${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
  BINARY_NAME="${BINARY_NAME}.exe"
fi

# Get latest release
echo "Fetching latest release..."
LATEST_RELEASE=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest")
TAG_NAME=$(echo "$LATEST_RELEASE" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$TAG_NAME" ]; then
  echo "Error: Could not determine latest release"
  exit 1
fi

echo "Latest version: $TAG_NAME"

# Download binary
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG_NAME}/${BINARY_NAME}"
echo "Downloading from: $DOWNLOAD_URL"

TMP_FILE=$(mktemp)
if ! curl -sL "$DOWNLOAD_URL" -o "$TMP_FILE"; then
  echo "Error: Failed to download ore-light"
  rm -f "$TMP_FILE"
  exit 1
fi

# Make executable
chmod +x "$TMP_FILE"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_FILE" "$INSTALL_DIR/ore"
  echo "✅ ore-light installed to $INSTALL_DIR/ore"
else
  echo "Installing to $INSTALL_DIR requires sudo..."
  sudo mv "$TMP_FILE" "$INSTALL_DIR/ore"
  echo "✅ ore-light installed to $INSTALL_DIR/ore"
fi

# Verify installation
if command -v ore >/dev/null 2>&1; then
  echo ""
  ore version
  echo ""
  echo "🎉 Installation complete!"
  echo "Try: ore --help"
else
  echo "⚠️  Installation complete but 'ore' not found in PATH"
  echo "Add $INSTALL_DIR to your PATH or run: $INSTALL_DIR/ore"
fi
