#!/bin/bash
set -e

INSTALL_DIR="$HOME/.local/bin"

# Check for Go
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed."
    echo "Install it from https://go.dev/dl/"
    exit 1
fi

echo "Building cocoon..."
cd "$(dirname "$0")"
go build -o cocoon cocoon.go

echo "Installing to $INSTALL_DIR..."
mkdir -p "$INSTALL_DIR"
mv cocoon "$INSTALL_DIR/cocoon"

# Check if install dir is on PATH
if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
    echo ""
    echo "Add this to your shell profile (~/.bashrc or ~/.zshrc):"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
fi

echo "Done. Run 'cocoon <directory>' to pack a sprite atlas."
